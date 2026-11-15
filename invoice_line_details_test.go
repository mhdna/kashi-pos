package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	db "kashi-pos/db/sqlc"
)

const invoiceLineDetailsGoldenPath = "frontend/src/lib/testdata/invoice_line_details.json"

// Two products named "Lady Leather Skirt" (TNV551219 and TNV551219-16) were
// told apart only by digging in central. The till's invoice screen shows
// every line's code, barcode and the rest of the product's details, so
// GetInvoiceDetails must carry them; a UI test reads the golden this writes.
func TestInvoiceDetailsCarryEveryProductDetail(t *testing.T) {
	app, cashbox, client, database := setupQuoteTestApp(t, 0, roundingHalfUp, "USD")
	inventory, err := app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []db.CreateProductParams{
		{Barcode: "8000000113603", Code: "TNV551219-16", Name: "Lady Leather Skirt", IsActive: 1, Price: 4500, Discount: 30},
		{Barcode: "8000000114129", Code: "TNV551219", Name: "Lady Leather Skirt", IsActive: 1, Price: 4500, Discount: 30},
	} {
		product, err := app.store.CreateProduct(app.ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: inventory.SalesInventoryID, ProductID: product.ID, Quantity: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`UPDATE products SET description = 'Skirts', color = 'Black', size = '16', brand = 'Tannav', kind = 'Leather', family = 'Bottoms', season = 'Winter', product_year = 2026 WHERE code = 'TNV551219-16'`); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)

	items, _ := json.Marshal([]map[string]any{{"code": "TNV551219-16", "qty": 2}, {"code": "TNV551219", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 9500, EntryType: "payment"}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	details, err := app.GetInvoiceDetails(sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	lines := details["items"].([]map[string]any)
	byCode := map[string]map[string]any{}
	for _, line := range lines {
		byCode[line["code"].(string)] = line
	}
	full, bare := byCode["TNV551219-16"], byCode["TNV551219"]
	if len(lines) != 2 || full == nil || bare == nil {
		t.Fatalf("lines = %#v", lines)
	}
	want := map[string]any{
		"name": "Lady Leather Skirt", "code": "TNV551219-16", "barcode": "8000000113603", "category": "Skirts",
		"color": "Black", "size": "16", "brand": "Tannav", "kind": "Leather", "family": "Bottoms",
		"season": "Winter", "productYear": int64(2026),
	}
	for key, value := range want {
		if full[key] != value {
			t.Errorf("full line %s = %#v, want %#v", key, full[key], value)
		}
	}
	if bare["barcode"] != "8000000114129" || bare["brand"] != "" || bare["productYear"] != int64(0) {
		t.Errorf("bare line = %#v", bare)
	}

	// The golden keeps only what the screen's detail row reads, in a fixed
	// order, so ids and timestamps don't churn it.
	keys := []string{"name", "code", "barcode", "category", "color", "size", "brand", "kind", "family", "season", "productYear"}
	golden := make([]map[string]any, 0, 2)
	for _, code := range []string{"TNV551219-16", "TNV551219"} {
		row := map[string]any{}
		for _, key := range keys {
			row[key] = byCode[code][key]
		}
		golden = append(golden, row)
	}
	got, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(invoiceLineDetailsGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantGolden, err := os.ReadFile(invoiceLineDetailsGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, wantGolden) {
		t.Fatalf("invoice line details changed; check the screen and rewrite %s with KASHI_UPDATE_GOLDEN=1:\n%s", invoiceLineDetailsGoldenPath, got)
	}
}
