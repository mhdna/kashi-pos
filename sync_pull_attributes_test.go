package main

import (
	"database/sql"
	"testing"
)

func TestSyncPullAppliesProductAttributes(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")

	v := syncedProductVariant{
		Barcode:     "9999999999999",
		Price:       sql.NullInt64{Int64: 1500, Valid: true},
		IsActive:    true,
		Code:        "ATTR-SKU",
		Name:        "Attribute Test Product",
		Description: "desc",
		Brand:       sql.NullString{String: "Nike", Valid: true},
		SubCategory: sql.NullString{String: "Outerwear", Valid: true},
		Kind:        sql.NullString{String: "Casual", Valid: true},
		Season:      sql.NullString{String: "Winter", Valid: true},
		Year:        sql.NullString{String: "2026", Valid: true},
	}
	if err := applySyncedProductVariant(app.ctx, store.Queries, v); err != nil {
		t.Fatal(err)
	}

	product, err := store.GetProductByBarcode(app.ctx, v.Barcode)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := loadOneProductAttributeValues(app.ctx, store.Queries, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attrs["brand"] != "Nike" || attrs["sub-category"] != "Outerwear" || attrs["kind"] != "Casual" ||
		attrs["season"] != "Winter" || attrs["year"] != "2026" {
		t.Fatalf("attributes not applied correctly: %+v", attrs)
	}

	// Re-applying with a changed brand must update the existing assignment,
	// not leave a stale one alongside a new one.
	v.Brand = sql.NullString{String: "Adidas", Valid: true}
	if err := applySyncedProductVariant(app.ctx, store.Queries, v); err != nil {
		t.Fatal(err)
	}
	attrs, err = loadOneProductAttributeValues(app.ctx, store.Queries, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attrs["brand"] != "Adidas" {
		t.Fatalf("brand not updated: %+v", attrs)
	}
}
