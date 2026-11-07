package main

import (
	"context"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// What an exchange comes to, worked out once for the screen (quoteExchange)
// and for checkout (checkoutReturnExchangeInvoice), so the total the cashier
// sees is the total that is charged.
//
//	net      = replacements - returned credit
//	tax      = the adjustment's tax on net (or the sale's own, see below)
//	rounding = what cash rounding added or took off
//	total    = net + tax + rounding
type exchangeTotals struct {
	net, tax, rounding, total int64
}

func computeExchangeTotals(ctx context.Context, q *db.Queries, source db.Invoice, priced pricedExchange, settings db.Setting, roundingMode string) (exchangeTotals, error) {
	var t exchangeTotals
	t.net = priced.replacementTotal - priced.returnedTotal
	if priced.replacementTotal == 0 && priced.returnedTotal == source.DiscountedTotal {
		// Returning every line of the invoice at full quantity, with
		// nothing bought in replacement: refund exactly what was charged
		// (source.GrandTotal) instead of recomputing tax from scratch at
		// today's rate, which would only coincidentally match the tax
		// actually charged at sale time.
		t.tax = -source.GrandTotal - t.net
	} else {
		// Partial return, or an exchange with a replacement: the
		// adjustment's tax at the configured rate.
		tax, err := signedAdjustmentTax(t.net, settings.TaxRate, roundingMode)
		if err != nil {
			return t, err
		}
		t.tax = tax
	}
	roundingCurrency, err := resolveCurrency(ctx, q, settings.RoundingCurrency)
	if err != nil {
		return t, fmt.Errorf("rounding currency: %w", err)
	}
	total, err := roundCentsByCurrency(t.net+t.tax, roundingCurrency.UnitsPerUsdMicros, roundingCurrency.CashRoundingUnit, roundingMode)
	if err != nil {
		return t, fmt.Errorf("round exchange total: %w", err)
	}
	t.total = total
	t.rounding = total - t.net - t.tax
	return t, nil
}

// ExchangeLineQuote is what one line of the exchange cart, as sent, comes
// to. Lines come back in the order they were sent (Index), one each, so the
// screen shows each cart line's own amount and the lines add up to the net.
//
// LineTotalCents is what the line adds to the exchange: positive for a
// replacement, negative for a returned line's credit. For a returned line,
//
//	-LineTotalCents = PaidCents + SwapCreditCents - ChargedBackCents
//
// PaidCents is what the customer paid for those units on the sale (after
// its discounts and its share of the invoice discount), SwapCreditCents the
// free unit's credit used on the replacements, ChargedBackCents the deal
// saving taken back because the deal is no longer earned (deal_exchange.go).
type ExchangeLineQuote struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Barcode string `json:"barcode"`
	// Signed as sent: below zero for a returned line.
	Qty            int64 `json:"qty"`
	LineTotalCents int64 `json:"lineTotalCents"`
	// The shelf price of the units (as sold, for a returned line), signed
	// like the line; what the line would be with nothing off.
	ShelfCents int64 `json:"shelfCents"`

	PaidCents        int64 `json:"paidCents,omitempty"`
	SwapCreditCents  int64 `json:"swapCreditCents,omitempty"`
	ChargedBackCents int64 `json:"chargedBackCents,omitempty"`
	// What a free unit could take off the replacements, used or not.
	SwapAllowanceCents int64  `json:"swapAllowanceCents,omitempty"`
	Deal               string `json:"deal,omitempty"`
	Free               bool   `json:"free,omitempty"`
}

// ExchangeAdjustmentQuote is what the exchange did to one of the sale's
// deals, said the way the receipt says it (dealAdjustmentNotes).
type ExchangeAdjustmentQuote struct {
	Deal             string `json:"deal"`
	ChargedBackCents int64  `json:"chargedBackCents,omitempty"`
	ForfeitedCents   int64  `json:"forfeitedCents,omitempty"`
}

// splitUnits shares amount over qty units as evenly as cents allow, the
// spare cents on the first units, so the units always add up to amount.
func splitUnits(amount, qty int64) []int64 {
	if qty <= 0 {
		return nil
	}
	units := make([]int64, qty)
	base, spare := amount/qty, amount%qty
	for i := range units {
		units[i] = base
		if int64(i) < spare {
			units[i]++
		} else if spare < 0 && int64(i) < -spare {
			units[i]--
		}
	}
	return units
}

// unitAmounts is one priced unit of a request group, by component.
type unitAmounts struct {
	total, shelf, paid, swap, chargedBack, allowance int64
	free                                             bool
	deal                                             string
}

// exchangeQuoteLines gives each sent line its share of what its returned
// item or replacement was priced at. A sent line can be merged with alike
// ones (parseExchangeRequest) and a priced one split (by price, by deal
// unit), so the priced units of each group are handed out, in order, to the
// group's sent lines by their quantities. Every component adds up exactly:
// nothing is recomputed, only shared out.
func exchangeQuoteLines(request exchangeRequest, priced pricedExchange, rawLines []exchangeLine, dealNames map[string]string) ([]ExchangeLineQuote, error) {
	returnedUnits := make([][]unitAmounts, len(request.returnedItems))
	for _, r := range priced.returned {
		if r.request < 0 || r.request >= len(returnedUnits) {
			return nil, fmt.Errorf("returned line %d is out of range", r.request)
		}
		if r.lineTotal != r.paid+r.swapCredit-r.chargedBack {
			return nil, fmt.Errorf("returned %s: credit %d is not paid %d + swap %d - charged back %d", r.product.Code, r.lineTotal, r.paid, r.swapCredit, r.chargedBack)
		}
		paid := splitUnits(r.paid, r.qty)
		swap := splitUnits(r.swapCredit, r.qty)
		back := splitUnits(r.chargedBack, r.qty)
		allowance := splitUnits(r.swapAllowance, r.qty)
		free := r.deal.unitDeal > 0 && r.deal.unitDeal == r.line.UnitPrice
		for i := range r.qty {
			returnedUnits[r.request] = append(returnedUnits[r.request], unitAmounts{
				total: paid[i] + swap[i] - back[i], shelf: r.line.UnitPrice,
				paid: paid[i], swap: swap[i], chargedBack: back[i], allowance: allowance[i],
				free: free, deal: dealNames[r.deal.dealUID],
			})
		}
	}
	replacementUnits := make([][]unitAmounts, len(request.replacements))
	for _, p := range priced.replacements {
		if p.exchangeRequest < 0 || p.exchangeRequest >= len(replacementUnits) {
			return nil, fmt.Errorf("replacement line %d is out of range", p.exchangeRequest)
		}
		totals := splitUnits(p.lineTotal, p.qty)
		for i := range p.qty {
			replacementUnits[p.exchangeRequest] = append(replacementUnits[p.exchangeRequest], unitAmounts{total: totals[i], shelf: p.unitPrice})
		}
	}

	if len(rawLines) != len(request.sources) {
		return nil, fmt.Errorf("%d lines sent, %d parsed", len(rawLines), len(request.sources))
	}
	lines := make([]ExchangeLineQuote, 0, len(request.sources))
	taken := map[bool][]int{true: make([]int, len(returnedUnits)), false: make([]int, len(replacementUnits))}
	for index, src := range request.sources {
		units := replacementUnits
		if src.returned {
			units = returnedUnits
		}
		group := units[src.group]
		from := taken[src.returned][src.group]
		if from+int(src.qty) > len(group) {
			return nil, fmt.Errorf("line %d (%s) has %d units priced, %d sent", index, rawLines[index].Code, len(group)-from, src.qty)
		}
		line := ExchangeLineQuote{Index: index, Code: rawLines[index].Code, Barcode: rawLines[index].Barcode, Qty: rawLines[index].Qty}
		free := true
		for _, u := range group[from : from+int(src.qty)] {
			line.LineTotalCents += u.total
			line.ShelfCents += u.shelf
			line.PaidCents += u.paid
			line.SwapCreditCents += u.swap
			line.ChargedBackCents += u.chargedBack
			line.SwapAllowanceCents += u.allowance
			free = free && u.free
			if line.Deal == "" {
				line.Deal = u.deal
			}
		}
		taken[src.returned][src.group] = from + int(src.qty)
		if src.returned {
			line.LineTotalCents, line.ShelfCents = -line.LineTotalCents, -line.ShelfCents
			line.Free = free && src.qty > 0
		}
		lines = append(lines, line)
	}
	for kind, groups := range map[bool][][]unitAmounts{true: returnedUnits, false: replacementUnits} {
		for g, group := range groups {
			if taken[kind][g] != len(group) {
				return nil, fmt.Errorf("%d priced units left over", len(group)-taken[kind][g])
			}
		}
	}

	// The lines must add up to the exchange, or the screen would show a sum
	// that isn't the total. Never quote one that doesn't.
	var sum int64
	for _, l := range lines {
		sum += l.LineTotalCents
	}
	if want := priced.replacementTotal - priced.returnedTotal; sum != want {
		return nil, fmt.Errorf("exchange lines add up to %d, the exchange is %d", sum, want)
	}
	return lines, nil
}

func exchangeAdjustmentQuotes(adjustments []exchangeDealAdjustment) []ExchangeAdjustmentQuote {
	out := make([]ExchangeAdjustmentQuote, 0, len(adjustments))
	for _, adj := range adjustments {
		out = append(out, ExchangeAdjustmentQuote{Deal: adj.dealName, ChargedBackCents: adj.clawback, ForfeitedCents: adj.forfeited})
	}
	return out
}
