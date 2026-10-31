package main

import (
	"context"
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// Invoice discounts (000132, kashi's 000212): a discount on a whole sale
// once it reaches a total -- "invoices of $150 or more get 15% off", or a
// fixed amount off. kashi decides the rules and which branches get them;
// the till keeps only those that apply to it.
//
// The total checked is the sale's after item discounts and deals, before
// tax, and the discount comes off that, on top of them. When several rules
// apply, the one that saves the customer most wins; they never add up.
//
// The discount is spread over the lines in proportion to what each costs
// and taken off their line totals (invoice_products.invoice_discount holds
// each share), so a return or an exchange credits what the line was
// actually paid. Exchanges don't earn one: their replacements are priced
// without it.

const (
	invoiceDiscountPercent = "percent"
	invoiceDiscountAmount  = "amount"
)

// syncedInvoiceDiscount is a rule as kashi sends it: every rule changed
// since the last pull, with whether it applies to this branch.
type syncedInvoiceDiscount struct {
	UID       string     `json:"uid"`
	Name      string     `json:"name"`
	MinTotal  int64      `json:"min_total"`
	Kind      string     `json:"kind"`
	Percent   int64      `json:"percent"`
	Amount    int64      `json:"amount"`
	ValidFrom *time.Time `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to"`
	Applies   bool       `json:"applies"`
}

func applySyncedInvoiceDiscount(ctx context.Context, q *db.Queries, item syncedInvoiceDiscount) error {
	if item.UID == "" {
		return fmt.Errorf("invoice discount without a uid")
	}
	if !item.Applies {
		return q.DeleteInvoiceDiscount(ctx, item.UID)
	}
	return q.UpsertInvoiceDiscount(ctx, db.UpsertInvoiceDiscountParams{
		Uid: item.UID, Name: item.Name, MinTotal: item.MinTotal, Kind: item.Kind,
		Percent: item.Percent, Amount: item.Amount,
		ValidFrom: rfc3339OrEmpty(item.ValidFrom), ValidTo: rfc3339OrEmpty(item.ValidTo),
	})
}

func rfc3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// invoiceDiscount is what a sale gets: the rule's name and the cents it
// takes off. The zero value is none.
type invoiceDiscount struct {
	Name  string
	Cents int64
}

// bestInvoiceDiscount picks, among the rules in force at now whose minimum
// the total reaches, the one that takes the most off it.
func bestInvoiceDiscount(rules []db.InvoiceDiscount, total int64, now time.Time, roundingMode string) invoiceDiscount {
	var best invoiceDiscount
	if total <= 0 {
		return best
	}
	for _, r := range rules {
		if total < r.MinTotal || !invoiceDiscountInForce(r, now) {
			continue
		}
		var cents int64
		switch r.Kind {
		case invoiceDiscountPercent:
			cents = percentOf(total, r.Percent, roundingMode)
		case invoiceDiscountAmount:
			cents = r.Amount
		}
		cents = min(cents, total)
		if cents > best.Cents {
			best = invoiceDiscount{Name: r.Name, Cents: cents}
		}
	}
	return best
}

// invoiceDiscountInForce: valid_to is the first moment the rule is over.
// A date the till can't read keeps the rule out rather than in.
func invoiceDiscountInForce(r db.InvoiceDiscount, now time.Time) bool {
	if r.ValidFrom != "" {
		from, err := time.Parse(time.RFC3339, r.ValidFrom)
		if err != nil || now.Before(from) {
			return false
		}
	}
	if r.ValidTo != "" {
		to, err := time.Parse(time.RFC3339, r.ValidTo)
		if err != nil || !now.Before(to) {
			return false
		}
	}
	return true
}

// spreadInvoiceDiscount takes cents off the lines in proportion to their
// totals. What rounding leaves goes to the dearest line, so the shares add
// up to cents exactly; no line goes below zero since cents never exceeds
// their sum.
func spreadInvoiceDiscount(products []pricedInvoiceProduct, cents int64) []pricedInvoiceProduct {
	var sum int64
	dearest := -1
	for i, p := range products {
		if p.lineTotal <= 0 {
			continue
		}
		sum += p.lineTotal
		if dearest < 0 || p.lineTotal > products[dearest].lineTotal {
			dearest = i
		}
	}
	if cents <= 0 || sum <= 0 {
		return products
	}
	cents = min(cents, sum)
	out := make([]pricedInvoiceProduct, len(products))
	copy(out, products)
	var given int64
	for i := range out {
		if out[i].lineTotal <= 0 {
			continue
		}
		share := cents * out[i].lineTotal / sum
		out[i].invoiceDiscount = share
		given += share
	}
	out[dearest].invoiceDiscount += cents - given
	for i := range out {
		out[i].lineTotal -= out[i].invoiceDiscount
	}
	return out
}

// applyInvoiceDiscount finds the sale's invoice discount and takes it off
// its lines and its discounted subtotal.
func (a *App) applyInvoiceDiscount(q *db.Queries, totals *invoiceTotals, roundingMode string) error {
	rules, err := q.ListInvoiceDiscounts(a.ctx)
	if err != nil {
		return fmt.Errorf("load invoice discounts: %w", err)
	}
	best := bestInvoiceDiscount(rules, totals.DiscountedSubtotal, time.Now(), roundingMode)
	if best.Cents <= 0 {
		return nil
	}
	totals.Products = spreadInvoiceDiscount(totals.Products, best.Cents)
	totals.DiscountedSubtotal -= best.Cents
	totals.InvoiceDiscount = best
	return nil
}

// invoiceDiscountLabel is the receipt row's name: the rule's, or a plain
// one for a rule saved without.
func invoiceDiscountLabel(d invoiceDiscount) string {
	if d.Name == "" {
		return "Invoice discount"
	}
	return d.Name
}

// returnedInvoiceDiscount is the part of a sold line's invoice discount
// that qty returned units carry, after earlier returns of it: a whole cent
// per unit, and the units that empty the line take what's left over, so
// returning all of it gives back exactly its share.
func returnedInvoiceDiscount(line db.InvoiceProduct, alreadyReturned, qty int64) int64 {
	if line.InvoiceDiscount <= 0 || line.Quantity <= 0 {
		return 0
	}
	perUnit := line.InvoiceDiscount / line.Quantity
	if alreadyReturned+qty >= line.Quantity {
		return line.InvoiceDiscount - perUnit*alreadyReturned
	}
	return perUnit * qty
}
