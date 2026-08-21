package main

import (
	"context"

	db "kashi-pos/db/sqlc"
)

// activePriceOverride/activeDiscountOverride are the resolved effect of an
// active price-list/discount-list entry on a product code.
type activePriceOverride struct {
	cents    int64
	listID   int64
	listName string
}

type activeDiscountOverride struct {
	percent  int64
	listID   int64
	listName string
}

// activeOverrides is every currently-active price-list/discount-list entry,
// keyed by product code. It is loaded once per pricing pass (checkout, a
// product listing/search page, a cart add) rather than once per product, so
// resolving overrides for N products costs 2 queries, not 2N.
type activeOverrides struct {
	prices    map[string]activePriceOverride
	discounts map[string]activeDiscountOverride
}

func loadActiveOverrides(ctx context.Context, q *db.Queries) (activeOverrides, error) {
	overrides := activeOverrides{
		prices:    map[string]activePriceOverride{},
		discounts: map[string]activeDiscountOverride{},
	}
	priceRows, err := q.ListActivePriceListItems(ctx)
	if err != nil {
		return overrides, err
	}
	for _, row := range priceRows {
		overrides.prices[row.ProductCode] = activePriceOverride{cents: row.Price, listID: row.ListID, listName: row.ListName}
	}
	discountRows, err := q.ListActiveDiscountListItems(ctx)
	if err != nil {
		return overrides, err
	}
	for _, row := range discountRows {
		overrides.discounts[row.ProductCode] = activeDiscountOverride{percent: row.Discount, listID: row.ListID, listName: row.ListName}
	}
	return overrides, nil
}
