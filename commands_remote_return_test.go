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

func TestExecuteRemoteReturnInvoiceCommand(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "remote_return.db")+"?_pragma=foreign_keys(ON)")
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
		rounding_mode, receipt_enabled, sales_inventory_id, central_api_url
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, 'http://kashi.test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateShift(ctx, db.CreateShiftParams{}); err != nil {
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
	salesperson, err := store.CreateSalesperson(ctx, "Remote Return Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "999", Code: "REMOTE-RETURN-1", Name: "Remote return product", IsActive: 1, Price: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, store.Queries)
	if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{
		InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5,
	}); err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SalesInventoryID = inventory.ID
	if err := store.UpdateSettings(ctx, db.UpdateSettingsParams{
		BranchName: settings.BranchName, BranchCode: settings.BranchCode,
		ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, RoundingCurrency: "USD", ReceiptEnabled: settings.ReceiptEnabled,
		SalesInventoryID: settings.SalesInventoryID, ExchangeWindowHours: settings.ExchangeWindowHours,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	saleItems, _ := json.Marshal([]salesInvoiceItem{{Code: product.Code, Qty: 2, SalespersonID: salesperson.ID}})
	salePayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 2000}})
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
	sourceID := loaded["id"].(int64)

	clientRef, err := store.GetInvoiceClientRef(ctx, sourceID)
	if err != nil {
		t.Fatalf("expected sale to already have a client_ref queued for sync: %v", err)
	}

	payload, _ := json.Marshal(remoteReturnInvoiceCommandPayload{
		ClientRef:       clientRef,
		BranchInvoiceID: 789,
		Reason:          "Customer changed mind",
	})

	if err := app.executeRemoteReturnInvoiceCommand(ctx, payload); err != nil {
		t.Fatalf("executeRemoteReturnInvoiceCommand: %v", err)
	}

	// Second time must fail: it's already fully returned.
	if err := app.executeRemoteReturnInvoiceCommand(ctx, payload); err == nil {
		t.Fatal("expected duplicate remote return to fail")
	}

	rows, err := database.QueryContext(ctx, `SELECT entity_type, client_ref FROM sync_outbox ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var entityType, ref string
		if err := rows.Scan(&entityType, &ref); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, entityType+":"+ref)
	}
	found := false
	for _, k := range kinds {
		if k == "return_invoice:"+clientRef {
			t.Fatalf("return_invoice outbox entry should reference the RETURN invoice's own client_ref, not the original sale's, got %v", kinds)
		}
		if len(k) > len("return_invoice:") && k[:len("return_invoice:")] == "return_invoice:" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a return_invoice sync_outbox entry, got %v", kinds)
	}
}
