package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestCustomInvoicePriceDoesNotChangeProductPrice(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "custom-price.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "123", Code: "SHIRT-1", Name: "Shirt", IsActive: 1,
		Price: 2500, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	customPrice := int64(1999)
	items := []salesInvoiceItem{{
		Code: product.Code, Qty: 1, CustomPriceCents: &customPrice,
	}}

	if _, _, _, _, err := app.validateAndPriceItems(
		store.Queries, 0, items, "half_up", false,
	); err == nil {
		t.Fatal("expected custom price to require the pricing feature")
	}

	priced, subtotal, discountedSubtotal, receiptItems, err := app.validateAndPriceItems(
		store.Queries, 0, items, "half_up", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(priced) != 1 || priced[0].unitPrice != customPrice {
		t.Fatalf("invoice unit price = %v, want %d", priced, customPrice)
	}
	if subtotal != customPrice || discountedSubtotal != customPrice {
		t.Fatalf(
			"invoice totals = %d/%d, want %d",
			subtotal, discountedSubtotal, customPrice,
		)
	}
	if len(receiptItems) != 1 || receiptItems[0].Price != 19.99 {
		t.Fatalf("receipt items = %#v, want price 19.99", receiptItems)
	}

	reloaded, err := store.GetProductByCode(ctx, product.Code)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Price != 2500 {
		t.Fatalf("stored product price = %d, want 2500", reloaded.Price)
	}
}
