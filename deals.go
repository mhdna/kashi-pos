package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	db "kashi-pos/db/sqlc"
)

// Deals (000128, kashi's 000201) price several units together: "Buy 2 get 1
// free", "Buy 1 get the 2nd 50% off", "3 for $50", "Buy 3 or more, 20% off
// all". A discount list row names the deal a variant is on; every unit in the
// cart on the same deal is pooled, whatever its product, and the cheapest
// units get the reward -- with six units on Buy 2 get 1 the two cheapest are
// free. A deal replaces a discount: a deal unit is priced from its list price.
//
// The till stores what the deal took off each line (deal_discount) and a copy
// of the deal's terms on the invoice (invoice_deals), so an exchange is
// judged by what was sold, not by the deal as it is today. See
// deal_exchange.go.

const (
	dealBuyGet        = "buy_get"
	dealBundlePrice   = "bundle_price"
	dealVolumePercent = "volume_percent"
)

type dealTerms struct {
	UID         string
	Name        string
	Kind        string
	BuyQty      int64
	GetQty      int64
	Percent     int64
	BundlePrice int64
	// What units outside a set get on a buy_get deal ("Buy 2 get 1, otherwise
	// 30%"); 0 for nothing.
	FallbackPercent int64
}

func dealTermsFromRow(d db.Deal) dealTerms {
	return dealTerms{UID: d.Uid, Name: d.Name, Kind: d.Kind, BuyQty: d.BuyQty, GetQty: d.GetQty, Percent: d.Percent, BundlePrice: d.BundlePrice, FallbackPercent: d.FallbackPercent}
}

func dealTermsFromInvoice(d db.InvoiceDeal) dealTerms {
	return dealTerms{UID: d.DealUid, Name: d.Name, Kind: d.Kind, BuyQty: d.BuyQty, GetQty: d.GetQty, Percent: d.Percent, BundlePrice: d.BundlePrice, FallbackPercent: d.FallbackPercent}
}

// rewardsFreeUnits says whether the deal's reward is particular units (Buy 2
// get 1: the free one), rather than a saving spread over every unit (3 for
// $50, 3 or more 20% off). Only a reward unit is swap credit at its shelf
// price in an exchange; see deal_exchange.go.
func (t dealTerms) rewardsFreeUnits() bool { return t.Kind == dealBuyGet }

// isRewardUnit says whether a sold unit, at its shelf price with the deal
// taking unitDeal off it, was the deal's reward rather than a unit that got
// the fallback discount (which is a saving, not a free unit, and earns no
// swap credit). The two are told apart by amount; a fallback equal to the
// reward's percent can't be, and counts as the reward.
func (t dealTerms) isRewardUnit(price, unitDeal int64, roundingMode string) bool {
	if !t.rewardsFreeUnits() || unitDeal <= 0 {
		return false
	}
	if t.FallbackPercent > 0 && t.FallbackPercent != t.Percent && unitDeal == percentOf(price, t.FallbackPercent, roundingMode) {
		return false
	}
	return true
}

// label is the deal's terms in a few characters, for the cart's chip:
// "B2G1", "B1G1 50%", "3 for $50", "3+ 20%".
func (t dealTerms) label() string {
	switch t.Kind {
	case dealBuyGet:
		label := fmt.Sprintf("B%dG%d", t.BuyQty, t.GetQty)
		if t.Percent < 100 {
			label += fmt.Sprintf(" %d%%", t.Percent)
		}
		if t.FallbackPercent > 0 {
			label += fmt.Sprintf(" else %d%%", t.FallbackPercent)
		}
		return label
	case dealBundlePrice:
		return fmt.Sprintf("%d for $%s", t.BuyQty, strings.TrimSuffix(centsString(t.BundlePrice), ".00"))
	case dealVolumePercent:
		return fmt.Sprintf("%d+ %d%%", t.BuyQty, t.Percent)
	}
	return t.Name
}

// unitDiscounts is what the deal takes off each of these units, given their
// shelf prices, in the same order. Units are taken cheapest first; equal
// prices keep their order, so the result never depends on map iteration.
func (t dealTerms) unitDiscounts(prices []int64, roundingMode string) []int64 {
	discounts := make([]int64, len(prices))
	order := make([]int, len(prices))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return prices[order[a]] < prices[order[b]] })
	n := int64(len(prices))

	switch t.Kind {
	case dealBuyGet:
		set := t.BuyQty + t.GetQty
		if set <= 0 || t.Percent <= 0 {
			return discounts
		}
		rewards := n / set * t.GetQty
		for _, i := range order[:rewards] {
			discounts[i] = percentOf(prices[i], t.Percent, roundingMode)
		}
		// The units outside a set (the dearest ones) get the fallback; a
		// full set is the deal alone, whatever the fallback would have saved.
		if t.FallbackPercent > 0 {
			for _, i := range order[n/set*set:] {
				discounts[i] = percentOf(prices[i], t.FallbackPercent, roundingMode)
			}
		}
	case dealBundlePrice:
		if t.BuyQty < 2 || t.BundlePrice <= 0 {
			return discounts
		}
		for start := int64(0); start+t.BuyQty <= n; start += t.BuyQty {
			bundle := order[start : start+t.BuyQty]
			var sum int64
			for _, i := range bundle {
				sum += prices[i]
			}
			if sum <= t.BundlePrice {
				continue // the bundle would cost more than its units do
			}
			// The saving is shared in proportion to price; what rounding
			// leaves goes to the dearest unit, so the bundle costs exactly
			// its price.
			saving, given := sum-t.BundlePrice, int64(0)
			for _, i := range bundle {
				discounts[i] = saving * prices[i] / sum
				given += discounts[i]
			}
			discounts[bundle[len(bundle)-1]] += saving - given
		}
	case dealVolumePercent:
		if t.BuyQty < 1 || n < t.BuyQty || t.Percent <= 0 {
			return discounts
		}
		for i, price := range prices {
			discounts[i] = percentOf(price, t.Percent, roundingMode)
		}
	}
	return discounts
}

// totalDiscount is the deal's whole saving on these units.
func (t dealTerms) totalDiscount(prices []int64, roundingMode string) int64 {
	var total int64
	for _, d := range t.unitDiscounts(prices, roundingMode) {
		total += d
	}
	return total
}

func percentOf(price, percent int64, roundingMode string) int64 {
	if percent >= 100 {
		return price
	}
	return roundRatio(price*percent, 100, roundingMode)
}

// applyDeals prices the deal lines of a cart. Lines naming a deal the till
// knows are pooled per deal; each unit gets its deal discount, and a line
// whose units come out differently is split, so a free unit is a line of its
// own. Lines not on a deal pass through untouched, in their place.
func applyDeals(products []pricedInvoiceProduct, deals map[string]dealTerms, roundingMode string) []pricedInvoiceProduct {
	type unit struct {
		line  int
		price int64
	}
	pools := map[string][]unit{}
	var poolOrder []string
	for i, p := range products {
		if p.dealUID == "" {
			continue
		}
		if _, ok := deals[p.dealUID]; !ok {
			products[i].dealUID = "" // a deal this till has no terms for gives nothing
			continue
		}
		if _, seen := pools[p.dealUID]; !seen {
			poolOrder = append(poolOrder, p.dealUID)
		}
		for range p.qty {
			pools[p.dealUID] = append(pools[p.dealUID], unit{line: i, price: p.unitPrice})
		}
	}
	if len(pools) == 0 {
		return products
	}

	// unitDeal[line] is the deal discount of each of that line's units.
	unitDeal := map[int][]int64{}
	for _, uid := range poolOrder {
		terms := deals[uid]
		units := pools[uid]
		prices := make([]int64, len(units))
		for i, u := range units {
			prices[i] = u.price
		}
		discounts := terms.unitDiscounts(prices, roundingMode)
		for i, u := range units {
			unitDeal[u.line] = append(unitDeal[u.line], discounts[i])
		}
	}

	out := make([]pricedInvoiceProduct, 0, len(products)+len(unitDeal))
	for i, p := range products {
		perUnit, onDeal := unitDeal[i]
		if !onDeal {
			out = append(out, p)
			continue
		}
		// One line per distinct unit discount, the full-price units first.
		counts := map[int64]int64{}
		for _, d := range perUnit {
			counts[d]++
		}
		amounts := make([]int64, 0, len(counts))
		for d := range counts {
			amounts = append(amounts, d)
		}
		sort.Slice(amounts, func(a, b int) bool { return amounts[a] < amounts[b] })
		terms := deals[p.dealUID]
		for _, d := range amounts {
			line := p
			line.qty = counts[d]
			line.dealName = terms.Name
			line.dealLabel = terms.label()
			line.dealDiscount = d * line.qty
			line.lineTotal = line.unitPrice*line.qty - line.dealDiscount
			// A unit the deal gave nothing (the pool fell short, or it paid
			// for the others' reward) still names its deal: it counts
			// towards it in an exchange, and the cart can say so.
			out = append(out, line)
		}
	}
	return out
}

// invoiceDealTerms is the terms of every deal the lines name, for the
// invoice's copy (invoice_deals).
func invoiceDealTerms(products []pricedInvoiceProduct, deals map[string]dealTerms) []dealTerms {
	var out []dealTerms
	seen := map[string]bool{}
	for _, p := range products {
		if p.dealUID != "" && !seen[p.dealUID] {
			seen[p.dealUID] = true
			out = append(out, deals[p.dealUID])
		}
	}
	return out
}

// isFree is a deal line whose units cost nothing.
func (p pricedInvoiceProduct) isFree() bool {
	return p.dealDiscount > 0 && p.lineTotal == 0
}

// recordInvoiceDeals copies the deals' terms onto the invoice.
func recordInvoiceDeals(ctx context.Context, q *db.Queries, invoiceID int64, deals []dealTerms) error {
	for _, d := range deals {
		if err := q.CreateInvoiceDeal(ctx, db.CreateInvoiceDealParams{
			InvoiceID: invoiceID, DealUid: d.UID, Name: d.Name, Kind: d.Kind,
			BuyQty: d.BuyQty, GetQty: d.GetQty, Percent: d.Percent, BundlePrice: d.BundlePrice,
			FallbackPercent: d.FallbackPercent,
		}); err != nil {
			return fmt.Errorf("record deal %s: %w", d.Name, err)
		}
	}
	return nil
}
