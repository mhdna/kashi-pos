package main

import (
	"context"

	db "kashi-pos/db/sqlc"
)

// Since 000097 a price is not a property of a product row -- it is an entry
// in a price list that kashi dictates and the sync mirrors down. products.price
// and products.discount still exist but are frozen: nothing reads them, and a
// product's sellable price comes from here or from nowhere.
//
// "From nowhere" is a real, intended state. A product in no active list has no
// price, and priceInvoiceProduct rejects a zero price, so the sale fails at the
// till rather than ringing something up for free. That is the whole point of
// the change: central decides what is sellable.

type activePrice struct {
	cents    int64
	listID   int64
	listName string
}

type activeDiscount struct {
	percent  int64
	listID   int64
	listName string
}

// activePricing is every price and discount currently in force, keyed by
// product uid. It is loaded once per pricing pass (checkout, a product
// listing/search page, a cart add) rather than once per product, so pricing N
// products costs 2 queries, not 2N.
type activePricing struct {
	prices    map[string]activePrice
	discounts map[string]activeDiscount
}

// priceFor / discountFor report whether central said anything about a product
// at all. The bool is the part that matters: false means unsellable, and a
// caller must never collapse it to a zero price.
func (p activePricing) priceFor(productUID string) (activePrice, bool) {
	v, ok := p.prices[productUID]
	return v, ok
}

func (p activePricing) discountFor(productUID string) (activeDiscount, bool) {
	v, ok := p.discounts[productUID]
	return v, ok
}

func loadActivePricing(ctx context.Context, q *db.Queries) (activePricing, error) {
	pricing := activePricing{
		prices:    map[string]activePrice{},
		discounts: map[string]activeDiscount{},
	}

	// Both queries order is_default DESC, so a default-list row is written
	// first and a branch-list row for the same product overwrites it -- the
	// branch's own list wins, and a product only the default mentions keeps
	// the default price. See ListActivePriceListItems.
	priceRows, err := q.ListActivePriceListItems(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range priceRows {
		pricing.prices[row.ProductUid] = activePrice{
			cents: row.Price, listID: row.ListID, listName: row.ListName,
		}
	}

	discountRows, err := q.ListActiveDiscountListItems(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range discountRows {
		pricing.discounts[row.ProductUid] = activeDiscount{
			percent: row.Discount, listID: row.ListID, listName: row.ListName,
		}
	}
	return pricing, nil
}
