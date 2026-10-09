package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

type cancelFixture struct {
	ctx         context.Context
	database    *sql.DB
	store       *db.Store
	app         *App
	productID   int64
	saleID      int64
	saleRef     string
}

// newCancelFixture is TestExecuteRemoteReturnInvoiceCommand's setup: a synced
// till with one sale of 2 units at $10 each.
func newCancelFixture(t *testing.T) cancelFixture {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "cancel.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
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
	salesperson, err := store.CreateSalesperson(ctx, "Cancel Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "999", Code: "CANCEL-1", Name: "Cancel product", IsActive: 1, Price: 1000,
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
	ref, err := store.GetInvoiceClientRef(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	return cancelFixture{ctx, database, store, app, product.ID, sale.InvoiceID, ref}
}

func TestRemoteCancelInvoice(t *testing.T) {
	f := newCancelFixture(t)
	payload, _ := json.Marshal(remoteReturnInvoiceCommandPayload{ClientRef: f.saleRef, Cancel: true})
	if err := f.app.executeRemoteReturnInvoiceCommand(f.ctx, payload); err != nil {
		t.Fatalf("remote cancel: %v", err)
	}

	var cancelled int
	if err := f.database.QueryRow(`SELECT COUNT(*) FROM invoices WHERE cancelled_at IS NOT NULL`).Scan(&cancelled); err != nil {
		t.Fatal(err)
	}
	if cancelled != 2 {
		t.Fatalf("cancelled invoices = %d, want 2 (the sale and its cancel return)", cancelled)
	}

	var body string
	if err := f.database.QueryRow(`SELECT payload_json FROM sync_outbox WHERE entity_type = 'return_invoice'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	var sent branchInvoicePayload
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatal(err)
	}
	if !sent.Cancellation || sent.RelatedClientRef != f.saleRef || sent.GrandTotal != -2000 {
		t.Fatalf("upload = cancellation %v, related %q, total %d; want true, %q, -2000",
			sent.Cancellation, sent.RelatedClientRef, sent.GrandTotal, f.saleRef)
	}

	if err := f.app.executeRemoteReturnInvoiceCommand(f.ctx, payload); err == nil {
		t.Fatal("cancelling an already cancelled sale must fail")
	}
	if err := f.app.DeleteInvoice(f.saleID); err == nil {
		t.Fatal("deleting a cancelled invoice must fail")
	}
}

func TestCancelRefusedAfterPartialReturn(t *testing.T) {
	f := newCancelFixture(t)
	partial, _ := json.Marshal(remoteReturnInvoiceCommandPayload{
		ClientRef: f.saleRef,
		Items:     []remoteReturnInvoiceItemPayload{{BranchProductID: f.productID, Quantity: 1}},
	})
	if err := f.app.executeRemoteReturnInvoiceCommand(f.ctx, partial); err != nil {
		t.Fatal(err)
	}
	err := f.app.cancelInvoice(f.ctx, f.saleID, "mistake")
	if err == nil || !strings.Contains(err.Error(), "already has a return") {
		t.Fatalf("cancel after a partial return: got %v, want the already-has-a-return refusal", err)
	}
}

func TestDeleteInvoiceOnlyBeforeUpload(t *testing.T) {
	f := newCancelFixture(t)
	if _, err := f.database.Exec(`UPDATE sync_outbox SET status = 'synced' WHERE client_ref = ?`, f.saleRef); err != nil {
		t.Fatal(err)
	}
	if err := f.app.DeleteInvoice(f.saleID); err == nil || !strings.Contains(err.Error(), "already uploaded") {
		t.Fatalf("delete after upload: got %v, want the already-uploaded refusal", err)
	}

	if _, err := f.database.Exec(`UPDATE sync_outbox SET status = 'pending' WHERE client_ref = ?`, f.saleRef); err != nil {
		t.Fatal(err)
	}
	if err := f.app.DeleteInvoice(f.saleID); err != nil {
		t.Fatalf("delete before upload: %v", err)
	}
	var left int
	if err := f.database.QueryRow(`SELECT COUNT(*) FROM sync_outbox WHERE client_ref = ?`, f.saleRef).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatal("the deleted invoice's pending upload must be dropped too")
	}
}
