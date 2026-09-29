package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestPerUnitInvoicePricesRequireFeatureAndGroupPriceLines(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "per-unit-price.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "BZ005", Code: "BZ005", Name: "Test item", IsActive: 1,
		Price: 1000, Discount: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, store.Queries)
	app := &App{ctx: ctx, store: store}
	items := []salesInvoiceItem{{
		Code: product.Code, Qty: 5,
		UnitPricesCents: []int64{1000, 1000, 750, 1000, 1000},
		UnitDiscounts:   []int64{10, 10, 0, 10, 10},
	}}

	if _, _, _, _, err := app.validateAndPriceItems(
		store.Queries, 0, items, "half_up", false, activePricing{}, false,
	); err == nil {
		t.Fatal("expected per-unit prices to require the feature")
	}

	priced, subtotal, discountedSubtotal, receiptItems, err := app.validateAndPriceItems(
		store.Queries, 0, items, "half_up", false, activePricing{}, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(priced) != 2 {
		t.Fatalf("priced lines = %d, want 2 price groups", len(priced))
	}
	if priced[0].unitPrice != 750 || priced[0].qty != 1 {
		t.Fatalf("custom price group = %#v, want one unit at 750", priced[0])
	}
	if priced[0].discount != 0 {
		t.Fatalf("custom price group discount = %d, want 0", priced[0].discount)
	}
	if priced[1].unitPrice != 1000 || priced[1].qty != 4 {
		t.Fatalf("regular price group = %#v, want four units at 1000", priced[1])
	}
	if subtotal != 4750 || discountedSubtotal != 4350 {
		t.Fatalf("totals = %d/%d, want 4750/4350", subtotal, discountedSubtotal)
	}
	if len(receiptItems) != 2 {
		t.Fatalf("receipt lines = %d, want 2", len(receiptItems))
	}
}

func TestParseSalesItemsValidatesPerUnitPriceCount(t *testing.T) {
	if _, err := parseSalesItems(
		`[{"code":"BZ005","qty":2,"unitPricesCents":[1000]}]`,
	); err == nil {
		t.Fatal("expected mismatched per-unit price count to fail")
	}
	if _, err := parseSalesItems(
		`[{"code":"BZ005","qty":2,"unitPricesCents":[1000,750],"unitDiscounts":[10,0]}]`,
	); err != nil {
		t.Fatalf("valid per-unit prices rejected: %v", err)
	}
	if _, err := parseSalesItems(
		`[{"code":"BZ005","qty":2,"unitPricesCents":[1000,750],"unitDiscounts":[10]}]`,
	); err == nil {
		t.Fatal("expected mismatched per-unit discount count to fail")
	}
}
