package main

import (
	"context"
	"database/sql"
	"testing"

	db "kashi-pos/db/sqlc"
)

// priceProductsFromRows puts every existing product into an active default
// price list (and discount list) at the price and discount its own row
// carries.
//
// Tests predate 000097: they create a product with Price/Discount columns and
// expect to sell it at those values. That is no longer how a price is found --
// products.price is frozen and pricing comes from the lists -- so without this
// every such test fails with "invalid pricing", which is the till correctly
// refusing to sell something central never priced.
//
// Calling this right after a test's products are created preserves each
// test's original intent: the number it chose is still the number charged.
func priceProductsFromRows(t *testing.T, q *db.Queries) {
	t.Helper()
	ctx := context.Background()

	priceList, err := q.UpsertPriceListByUID(ctx, db.UpsertPriceListByUIDParams{
		Uid:       sql.NullString{String: "test-default-price-list", Valid: true},
		Name:      "Default",
		IsActive:  1,
		IsDefault: 1,
	})
	if err != nil {
		t.Fatalf("seed default price list: %v", err)
	}
	discountList, err := q.UpsertDiscountListByUID(ctx, db.UpsertDiscountListByUIDParams{
		Uid:       sql.NullString{String: "test-default-discount-list", Valid: true},
		Name:      "Default",
		IsActive:  1,
		IsDefault: 1,
	})
	if err != nil {
		t.Fatalf("seed default discount list: %v", err)
	}

	products, err := q.ListProductsPageUnfiltered(ctx, db.ListProductsPageUnfilteredParams{
		Limit: 10000, Offset: 0,
	})
	if err != nil {
		t.Fatalf("list products to price: %v", err)
	}
	for _, p := range products {
		if !p.Uid.Valid {
			continue
		}
		if err := q.InsertPriceListItem(ctx, db.InsertPriceListItemParams{
			PriceListID: priceList.ID,
			ProductUid:  p.Uid.String,
			Price:       p.Price,
		}); err != nil {
			t.Fatalf("price product %s: %v", p.Code, err)
		}
		if p.Discount > 0 {
			if err := q.InsertDiscountListItem(ctx, db.InsertDiscountListItemParams{
				DiscountListID: discountList.ID,
				ProductUid:     p.Uid.String,
				Discount:       p.Discount,
			}); err != nil {
				t.Fatalf("discount product %s: %v", p.Code, err)
			}
		}
	}
}
