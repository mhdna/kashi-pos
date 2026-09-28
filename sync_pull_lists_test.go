package main

import (
	"context"
	"database/sql"
	"testing"
)

func syncedListWith(uid string, items ...syncedPriceListItem) syncedPriceList {
	return syncedPriceList{
		UID: uid, Name: "Default", IsActive: true, IsDefault: true, Items: items,
	}
}

// The reason lists are sent whole: an item kashi drops has no tombstone, so
// the only thing that can remove it locally is applying the list as a
// replace. This is the regression guard for that.
func TestApplySyncedPriceListRemovesItemsMissingFromThePayload(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	first := syncedListWith("central-uid-1",
		syncedPriceListItem{ProductVariantUID: "var-a", Price: 100},
		syncedPriceListItem{ProductVariantUID: "var-b", Price: 200},
	)
	if err := applySyncedPriceList(ctx, q, first); err != nil {
		t.Fatal(err)
	}

	list, err := q.GetPriceListByUID(ctx, sql.NullString{String: "central-uid-1", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	items, err := q.ListPriceListItems(ctx, list.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items after the first apply, got %d", len(items))
	}

	// kashi drops var-b and reprices var-a.
	second := syncedListWith("central-uid-1",
		syncedPriceListItem{ProductVariantUID: "var-a", Price: 150},
	)
	if err := applySyncedPriceList(ctx, q, second); err != nil {
		t.Fatal(err)
	}

	items, err = q.ListPriceListItems(ctx, list.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("removed item was not dropped locally: %+v", items)
	}
	if items[0].ProductUid != "var-a" || items[0].Price != 150 {
		t.Fatalf("surviving item wrong: %+v", items[0])
	}
}

// Applying the same payload twice must not duplicate anything -- the pull
// cursor only advances after a whole successful walk, so a mid-page failure
// replays the page.
func TestApplySyncedPriceListIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	list := syncedListWith("central-uid-2",
		syncedPriceListItem{ProductVariantUID: "var-a", Price: 100},
	)
	for range 3 {
		if err := applySyncedPriceList(ctx, q, list); err != nil {
			t.Fatal(err)
		}
	}

	lists, err := q.ListPriceLists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 {
		t.Fatalf("expected exactly one list, got %d", len(lists))
	}
	items, err := q.ListPriceListItems(ctx, lists[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly one item, got %d", len(items))
	}
}

// A list kashi deactivates is dropped outright, so it stops pricing anything.
func TestApplySyncedPriceListDeletesOnDeactivation(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	live := syncedListWith("central-uid-3",
		syncedPriceListItem{ProductVariantUID: "var-a", Price: 100},
	)
	if err := applySyncedPriceList(ctx, q, live); err != nil {
		t.Fatal(err)
	}

	retired := live
	retired.IsActive = false
	retired.Items = nil
	if err := applySyncedPriceList(ctx, q, retired); err != nil {
		t.Fatal(err)
	}

	if _, err := q.GetPriceListByUID(ctx, sql.NullString{String: "central-uid-3", Valid: true}); err == nil {
		t.Fatal("a deactivated list must not survive locally")
	}
	// Its items go with it (ON DELETE CASCADE), so nothing is left pricing.
	active, err := q.ListActivePriceListItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("retired list still prices products: %+v", active)
	}
}

// A branch's own list wins over the default for a product both mention, and
// the default still covers products the branch list is silent about.
func TestActivePricingPrefersBranchListOverDefault(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	if err := applySyncedPriceList(ctx, q, syncedPriceList{
		UID: "default-list", Name: "Default", IsActive: true, IsDefault: true,
		Items: []syncedPriceListItem{
			{ProductVariantUID: "var-a", Price: 100},
			{ProductVariantUID: "var-b", Price: 300},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{
		UID: "branch-list", Name: "Beirut", IsActive: true, IsDefault: false,
		Items: []syncedPriceListItem{
			{ProductVariantUID: "var-a", Price: 250},
		},
	}); err != nil {
		t.Fatal(err)
	}

	pricing, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := pricing.priceFor("var-a")
	if !ok || got.cents != 250 {
		t.Fatalf("branch list should win for var-a, got %+v (found=%v)", got, ok)
	}
	got, ok = pricing.priceFor("var-b")
	if !ok || got.cents != 300 {
		t.Fatalf("default should still cover var-b, got %+v (found=%v)", got, ok)
	}
	if _, ok := pricing.priceFor("var-unknown"); ok {
		t.Fatal("a product in no list must report no price at all")
	}
}

func TestApplySyncedDiscountListReplacesItems(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "dl-1", Name: "Sep Latest", IsActive: true, IsDefault: true,
		Items: []syncedDiscountListItem{
			{ProductVariantUID: "var-a", Discount: 20},
			{ProductVariantUID: "var-b", Discount: 50},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "dl-1", Name: "Sep Latest", IsActive: true, IsDefault: true,
		Items: []syncedDiscountListItem{
			{ProductVariantUID: "var-a", Discount: 35},
		},
	}); err != nil {
		t.Fatal(err)
	}

	pricing, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := pricing.discountFor("var-a")
	if !ok || got.percent != 35 {
		t.Fatalf("var-a discount = %+v (found=%v), want 35", got, ok)
	}
	if _, ok := pricing.discountFor("var-b"); ok {
		t.Fatal("var-b was dropped from the payload and must lose its discount")
	}
}

// A list arriving with no uid is refused rather than silently creating an
// unmatchable local row that no later sync could ever update.
func TestApplySyncedPriceListRejectsMissingUID(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)

	err := applySyncedPriceList(ctx, store.Queries, syncedPriceList{
		Name: "No UID", IsActive: true,
	})
	if err == nil {
		t.Fatal("expected an error for a list with no uid")
	}
}
