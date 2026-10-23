package main

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	db "kashi-pos/db/sqlc"
)

// The cache is reused while nothing that prices a sale changed, and dropped
// the moment something did -- whatever wrote it.
func TestLoadActivePricingCacheFollowsEveryWrite(t *testing.T) {
	ctx := context.Background()
	_, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries
	resetPricingCache()

	list := syncedPriceList{
		UID: "price-cache", Name: "Cache prices", IsActive: true,
		Items: []syncedPriceListItem{{ProductVariantUID: "var-cache", Price: 1000}},
	}
	if err := applySyncedPriceList(ctx, q, list); err != nil {
		t.Fatal(err)
	}

	first, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	again, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(first.prices).Pointer() != reflect.ValueOf(again.prices).Pointer() {
		t.Fatal("nothing changed, but the pricing was read again")
	}

	// The sync reprices the item.
	list.Items[0].Price = 1500
	if err := applySyncedPriceList(ctx, q, list); err != nil {
		t.Fatal(err)
	}
	after, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := after.priceFor("var-cache"); got.cents != 1500 {
		t.Fatalf("a repriced item came back at %d from the cache", got.cents)
	}

	// A single-row write outside the sync's apply still invalidates it: the
	// trigger, not the caller, moves the version.
	priceList, err := q.GetPriceListByUID(ctx, sql.NullString{String: "price-cache", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.InsertPriceListItem(ctx, db.InsertPriceListItemParams{
		PriceListID: priceList.ID, ProductUid: "var-cache", Price: 1700,
	}); err != nil {
		t.Fatal(err)
	}
	after, err = loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := after.priceFor("var-cache"); got.cents != 1700 {
		t.Fatalf("a raw SQL repricing came back at %d from the cache", got.cents)
	}
}
