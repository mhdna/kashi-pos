package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestExchangeLoadsInvoiceAndPreventsDuplicateReturn(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "exchange.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, receipt_enabled, sales_inventory_id
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateShift(ctx, 0); err != nil {
		t.Fatal(err)
	}
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	returnedProduct, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "111", Code: "RETURN-1", Name: "Returned product", IsActive: 1, Price: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	replacementProduct, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "222", Code: "REPLACE-1", Name: "Replacement product", IsActive: 1, Price: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range []db.Product{returnedProduct, replacementProduct} {
		if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{
			InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5,
		}); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SalesInventoryID = inventory.ID
	if err := store.UpdateSettings(ctx, db.UpdateSettingsParams{
		BranchName: settings.BranchName, ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, ReceiptEnabled: settings.ReceiptEnabled,
		SalesInventoryID: settings.SalesInventoryID,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	saleItems, _ := json.Marshal([]salesInvoiceItem{{Code: returnedProduct.Code, Qty: 1}})
	salePayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 1000}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{
		ItemsJSON: string(saleItems), PaymentsJSON: string(salePayments), ClientID: client.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := app.GetExchangeInvoice(sale.InvoiceCode)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := loaded["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("loaded items = %#v", loaded["items"])
	}
	if items[0]["code"] != returnedProduct.Code || items[0]["barcode"] != returnedProduct.Barcode {
		t.Fatalf("loaded item = %#v", items[0])
	}

	sourceID := loaded["id"].(int64)
	exchangeItems, _ := json.Marshal([]exchangeLine{
		{Code: returnedProduct.Code, Qty: -1},
		{Code: replacementProduct.Code, Qty: 1},
	})
	if _, err := app.CheckoutExchangeInvoice(string(exchangeItems), "[]", sourceID, 0); err != nil {
		t.Fatal(err)
	}

	returnedStock, err := store.GetInventoryItem(ctx, db.GetInventoryItemParams{
		InventoryID: inventory.ID, ProductID: returnedProduct.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	replacementStock, err := store.GetInventoryItem(ctx, db.GetInventoryItemParams{
		InventoryID: inventory.ID, ProductID: replacementProduct.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if returnedStock.Quantity != 5 || replacementStock.Quantity != 4 {
		t.Fatalf("stock after exchange: returned=%d replacement=%d", returnedStock.Quantity, replacementStock.Quantity)
	}

	if _, err := app.GetExchangeInvoice(sale.InvoiceCode); err == nil {
		t.Fatal("expected fully returned invoice to have no available items")
	}
	if _, err := app.CheckoutExchangeInvoice(string(exchangeItems), "[]", sourceID, 0); err == nil {
		t.Fatal("expected duplicate exchange to fail")
	}
}
