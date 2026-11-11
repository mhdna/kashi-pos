package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestSyncReportEntities(t *testing.T) {
	var r syncReporter
	boom := errors.New("FOREIGN KEY constraint failed")

	got := r.syncReportEntities([]syncPullResult{
		{entity: "currencies"},
		{entity: "products", applied: 12},
		{entity: "discount_lists", err: boom},
	})
	if len(got) != 2 || got[0].Entity != "products" || got[0].RowsApplied != 12 || got[1].Error != boom.Error() {
		t.Fatalf("first pass = %+v", got)
	}

	// The same error again is old news, and nothing else happened.
	if got := r.syncReportEntities([]syncPullResult{{entity: "discount_lists", err: boom}}); len(got) != 0 {
		t.Fatalf("repeat = %+v", got)
	}

	// Fixed: central hears it has cleared, even with nothing applied.
	got = r.syncReportEntities([]syncPullResult{{entity: "discount_lists"}})
	if len(got) != 1 || got[0].Error != "" {
		t.Fatalf("cleared = %+v", got)
	}
	if got := r.syncReportEntities([]syncPullResult{{entity: "discount_lists"}}); len(got) != 0 {
		t.Fatalf("after clearing = %+v", got)
	}
}

// The same rows must give the same hash here and in central: its
// TestListFingerprintVector checks these very values.
func TestListFingerprintMatchesKashi(t *testing.T) {
	a, b := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	deal := "33333333-3333-4333-8333-333333333333"

	rows, hash := discountListFingerprint(
		[]db.DiscountListItem{
			{ProductUid: b, DealUid: deal, DealFallbackPercent: sql.NullInt64{Int64: 30, Valid: true}},
			{ProductUid: a, Discount: 21},
		},
		[]db.DiscountListOptionItem{{ProductUid: a, SaleOption: "Winter", Discount: 20}},
	)
	if rows != 3 || hash != "e26eb35c5243c924941d9c727c36f816bc8719c8f8f3c93f45cc09b96ebdf9f0" {
		t.Fatalf("discount list: %d rows, %s", rows, hash)
	}
	rows, hash = priceListFingerprint([]db.PriceListItem{{ProductUid: b, Price: 1900}, {ProductUid: a, Price: 500}})
	if rows != 2 || hash != "151e829b427adfbd99b1a5c9a150810ba7f380a8f69ba71a3946d69ddc1c16ed" {
		t.Fatalf("price list: %d rows, %s", rows, hash)
	}
}

// A till fingerprints the lists in force, as they stand after a pull: a change shows,
// and a list central switched off leaves the report.
func TestSyncReportListsFollowTheTill(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries
	app := &App{}
	list := syncedDiscountList{UID: "d1", Name: "Deals", IsActive: true, Items: []syncedDiscountListItem{{ProductVariantUID: "x", Discount: 30}}}
	if err := applySyncedDiscountList(ctx, q, list); err != nil {
		t.Fatal(err)
	}
	first, err := app.syncReportLists(ctx, q)
	if err != nil || len(first) != 1 || first[0].UID != "d1" || first[0].Rows != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	list.Items[0].Discount = 21
	if err := applySyncedDiscountList(ctx, q, list); err != nil {
		t.Fatal(err)
	}
	second, err := app.syncReportLists(ctx, q)
	if err != nil || len(second) != 1 || second[0].Hash == first[0].Hash {
		t.Fatalf("a changed discount left the hash alone: %+v, %v", second, err)
	}
	list.IsActive, list.Items = false, nil
	if err := applySyncedDiscountList(ctx, q, list); err != nil {
		t.Fatal(err)
	}
	if third, err := app.syncReportLists(ctx, q); err != nil || len(third) != 0 {
		t.Fatalf("after switching off = %+v, %v", third, err)
	}
}
