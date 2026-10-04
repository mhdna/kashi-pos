package main

import (
	"errors"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestProductsCSVImportPricesNewProductsAndSkipsExisting(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	if _, err := app.CreateProduct(ProductInput{Barcode: "OLD-1", Name: "Existing", PriceCents: 100, IsActive: true}); err != nil {
		t.Fatal(err)
	}

	csvText := "Barcode,Name,Price,Discount,Color,IsActive\n" +
		"OLD-1,Existing again,9.00,0,,1\n" +
		"NEW-1,Shirt,12.50,20,Red,1\n" +
		"NEW-1,Shirt dup,13.00,0,,1\n" +
		"NEW-2,Hat,0,0,,1\n" +
		"NEW-3,Scarf,1,250.00,,0\n" +
		",No barcode,5,0,,1\n"
	preview, rows, err := app.previewProductsCSV(strings.NewReader(csvText))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Total != 6 || preview.New != 1 || preview.Existing != 1 || preview.Invalid != 4 {
		t.Fatalf("unexpected preview %+v", preview)
	}

	app.pendingProductsCSV = rows
	result, err := app.ImportPendingProductsCSV()
	if err != nil || result.Imported != 1 || result.Failed != 0 {
		t.Fatalf("import: %+v err=%v", result, err)
	}

	product, err := store.GetProductByBarcode(app.ctx, "NEW-1")
	if err != nil || product.Code != "NEW-1" || product.Color != "Red" {
		t.Fatalf("imported product: %+v err=%v", product, err)
	}
	pricing, err := loadActivePricing(app.ctx, store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	if price, ok := pricing.priceFor(product.Uid.String); !ok || price.cents != 1250 {
		t.Fatalf("expected a sellable list price of 1250, got %+v ok=%v", price, ok)
	}
	if discount, ok := pricing.discountFor(product.Uid.String); !ok || discount.percent != 20 {
		t.Fatalf("expected a 20%% list discount, got %+v ok=%v", discount, ok)
	}

	exported, err := app.productsCSVRows()
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, row := range exported[1:] {
		if row[0] == "NEW-1" {
			found = row
		}
	}
	if found == nil || found[6] != "12.50" || found[7] != "20" {
		t.Fatalf("exported row for NEW-1: %v", found)
	}

	if _, err := app.ImportPendingProductsCSV(); err == nil {
		t.Fatal("a second import without a new preview must be refused")
	}
}

func TestProductsCSVImportRefusedWhenConnected(t *testing.T) {
	app, _, _, _ := setUpTestInvoiceApp(t, "http://kashi.invalid")
	if _, err := app.PreviewProductsCSV(); !errors.Is(err, errManagedByKashi) {
		t.Fatalf("PreviewProductsCSV: expected errManagedByKashi, got %v", err)
	}
	if _, err := app.ImportPendingProductsCSV(); !errors.Is(err, errManagedByKashi) {
		t.Fatalf("ImportPendingProductsCSV: expected errManagedByKashi, got %v", err)
	}
}

func TestQuickAccessCSVExportKeepsOrderAndColors(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	var ids []int64
	for _, barcode := range []string{"QA-1", "QA-2"} {
		created, err := app.CreateProduct(ProductInput{Barcode: barcode, Name: barcode, PriceCents: 100, IsActive: true})
		if err != nil {
			t.Fatal(err)
		}
		id := created["id"].(int64)
		ids = append(ids, id)
		if err := store.AddQuickAccessProduct(app.ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetQuickAccessColor(app.ctx, db.SetQuickAccessColorParams{Color: "#c62828", TextColor: "#ffffff", ProductID: ids[1]}); err != nil {
		t.Fatal(err)
	}

	rows, err := app.quickAccessCSVRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][1] != "QA-1" || rows[2][1] != "QA-2" {
		t.Fatalf("unexpected rows %v", rows)
	}
	if rows[2][7] != "#c62828" || rows[2][8] != "#ffffff" || rows[1][7] != "" {
		t.Fatalf("colors not exported: %v", rows)
	}
}
