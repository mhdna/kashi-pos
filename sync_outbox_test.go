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

func setUpTestInvoiceApp(t *testing.T, centralAPIURL string) (*App, *db.Store, db.CashboxAccount, db.Client) {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "invoice.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	_ = shift
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Test Customer", Phone: "+96170123456"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 100, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSettings(ctx, db.UpdateSettingsParams{
		BranchName: settings.BranchName, BranchCode: "TEST01",
		ApiKey: "test-key", PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, RoundingCurrency: "USD", ReceiptEnabled: 0,
		SalesInventoryID: inventory.ID, ExchangeWindowHours: settings.ExchangeWindowHours,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCentralAPIURL(ctx, centralAPIURL); err != nil {
		t.Fatal(err)
	}

	return &App{ctx: ctx, store: store}, store, cashbox, client
}

func TestSalesInvoiceDoesNotQueueSyncWhenNotConfigured(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "")

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no outbox entries when sync isn't configured, got %d", count)
	}
}

func TestSalesInvoiceQueuesBranchInvoiceSyncWithReportedLoyaltyDelta(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 outbox entry, got %d", len(entries))
	}
	if entries[0].EntityType != "sales_invoice" {
		t.Fatalf("entity_type = %q, want sales_invoice", entries[0].EntityType)
	}

	var payload branchInvoicePayload
	if err := json.Unmarshal([]byte(entries[0].PayloadJson), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.GrandTotal != result.GrandTotal {
		t.Fatalf("payload grand_total = %d, want %d", payload.GrandTotal, result.GrandTotal)
	}
	wantLoyalty := result.GrandTotal / 100
	if payload.LoyaltyPointsDelta != wantLoyalty {
		t.Fatalf("payload loyalty_points_delta = %d, want %d (must match this install's own formula, not be recomputed centrally)", payload.LoyaltyPointsDelta, wantLoyalty)
	}
	if payload.BranchInvoiceCode != result.InvoiceCode {
		t.Fatalf("payload branch_invoice_code = %q, want %q", payload.BranchInvoiceCode, result.InvoiceCode)
	}
	if payload.ClientRef == "" {
		t.Fatal("expected a non-empty client_ref to be generated")
	}
	if len(payload.Items) != 1 || payload.Items[0].BranchProductID == 0 {
		t.Fatalf("expected 1 item with a branch_product_id, got %+v", payload.Items)
	}
}

func TestReturnInvoiceQueuesSyncLinkedToOriginalSale(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	salesperson, err := store.CreateSalesperson(app.ctx, "Return Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "2", Code: "SKU-2", Name: "Replacement", IsActive: 1, Price: 100, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: settings.SalesInventoryID, ProductID: replacement.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}

	saleItems, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	salePayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(saleItems), PaymentsJSON: string(salePayments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}

	// The sale itself should have queued entry #1 — confirm before adding
	// the return, so we can tell the two outbox entries apart below.
	beforeReturn, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeReturn) != 1 {
		t.Fatalf("expected 1 outbox entry after the sale, got %d", len(beforeReturn))
	}
	var salePayload branchInvoicePayload
	if err := json.Unmarshal([]byte(beforeReturn[0].PayloadJson), &salePayload); err != nil {
		t.Fatal(err)
	}

	returnItems, _ := json.Marshal([]exchangeLine{
		{Code: "SKU-1", Qty: -1},
		{Code: replacement.Code, Qty: 1},
	})
	if _, err := app.CheckoutExchangeInvoice(string(returnItems), "[]", sale.InvoiceID, 0, salesperson.ID, ""); err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 outbox entries (sale + return), got %d", len(entries))
	}
	var returnPayload branchInvoicePayload
	if err := json.Unmarshal([]byte(entries[1].PayloadJson), &returnPayload); err != nil {
		t.Fatal(err)
	}
	if entries[1].EntityType != "return_invoice" {
		t.Fatalf("entity_type = %q, want return_invoice", entries[1].EntityType)
	}
	if returnPayload.RelatedClientRef != salePayload.ClientRef {
		t.Fatalf("return related_client_ref = %q, want %q (the original sale's client_ref)", returnPayload.RelatedClientRef, salePayload.ClientRef)
	}
	if returnPayload.LoyaltyPointsDelta != 0 {
		t.Fatalf("return loyalty_points_delta = %d, want 0 (checkoutReturnExchangeInvoice doesn't touch loyalty points)", returnPayload.LoyaltyPointsDelta)
	}
	if len(returnPayload.Items) != 2 {
		t.Fatalf("expected 2 items (returned + replacement), got %+v", returnPayload.Items)
	}
	if returnPayload.Items[0].Quantity != -1 {
		t.Fatalf("returned item quantity = %d, want -1", returnPayload.Items[0].Quantity)
	}
	if returnPayload.Items[1].Quantity != 1 {
		t.Fatalf("replacement item quantity = %d, want 1", returnPayload.Items[1].Quantity)
	}
}
