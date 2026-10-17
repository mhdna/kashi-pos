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

// remoteReturnFixture is a till with one sale of 2 x product, ready for a
// remote command against it.
type remoteReturnFixture struct {
	app       *App
	store     *db.Store
	database  *sql.DB
	product   db.Product
	cashbox   db.CashboxAccount
	sourceID  int64
	clientRef string
}

func newRemoteReturnFixture(t *testing.T, extraProducts ...db.CreateProductParams) remoteReturnFixture {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "remote_return.db")+"?_pragma=foreign_keys(ON)")
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
	salesperson, err := store.CreateSalesperson(ctx, "Remote Return Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	// Extra products come first, so a variant sharing the sold product's
	// code sits ahead of it, the way an earlier colour or size would.
	for _, extra := range extraProducts {
		if _, err := store.CreateProduct(ctx, extra); err != nil {
			t.Fatal(err)
		}
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
	saleItems, _ := json.Marshal([]salesInvoiceItem{{Code: product.Code, Barcode: product.Barcode, Qty: 2, SalespersonID: salesperson.ID}})
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

	return remoteReturnFixture{app: app, store: store, database: database, product: product, cashbox: cashbox, sourceID: sourceID, clientRef: clientRef}
}

func TestExecuteRemoteReturnInvoiceCommand(t *testing.T) {
	ctx := context.Background()
	f := newRemoteReturnFixture(t)
	app, database, clientRef := f.app, f.database, f.clientRef

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

// An exchange from kashi takes one unit back and hands out a different
// variant of the same product, found by its uid. The sold variant isn't the
// first with its code, so the return has to go by barcode to find it.
func TestExecuteRemoteReturnInvoiceCommand_Exchange(t *testing.T) {
	ctx := context.Background()
	f := newRemoteReturnFixture(t, db.CreateProductParams{
		Barcode: "998", Code: "REMOTE-RETURN-1", Name: "Remote return product (red)", IsActive: 1, Price: 1500,
	})
	replacement, err := f.store.GetProductByBarcode(ctx, "998")
	if err != nil {
		t.Fatal(err)
	}
	uid := replacement.Uid.String // stamped when the fixture priced it

	payload, _ := json.Marshal(remoteReturnInvoiceCommandPayload{
		ClientRef:    f.clientRef,
		Items:        []remoteReturnInvoiceItemPayload{{BranchProductID: f.product.ID, Quantity: 1}},
		Replacements: []remoteReplacementPayload{{ProductUID: uid, Barcode: "998", Quantity: 1}},
	})
	if err := f.app.executeRemoteReturnInvoiceCommand(ctx, payload); err != nil {
		t.Fatalf("remote exchange: %v", err)
	}

	var exchangeID int64
	if err := f.database.QueryRowContext(ctx,
		`SELECT invoice_id FROM return_invoices WHERE sales_invoice_id = ?`, f.sourceID).Scan(&exchangeID); err != nil {
		t.Fatal(err)
	}
	lines, err := f.store.ListInvoiceProducts(ctx, exchangeID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]int64{}
	for _, line := range lines {
		got[line.ProductID] = line.Quantity
	}
	if got[f.product.ID] != -1 || got[replacement.ID] != 1 || len(got) != 2 {
		t.Fatalf("exchange lines = %v, want %d:-1 and %d:+1", got, f.product.ID, replacement.ID)
	}

	// The difference ($5 more for the red one) lands on the sale's account.
	var grandTotal int64
	if err := f.database.QueryRowContext(ctx, `SELECT grand_total FROM invoices WHERE id = ?`, exchangeID).Scan(&grandTotal); err != nil {
		t.Fatal(err)
	}
	if grandTotal != 500 {
		t.Fatalf("exchange grand total = %d, want 500", grandTotal)
	}
}

func TestExecuteRemoteReturnInvoiceCommand_UnknownReplacementFails(t *testing.T) {
	ctx := context.Background()
	f := newRemoteReturnFixture(t)
	payload, _ := json.Marshal(remoteReturnInvoiceCommandPayload{
		ClientRef:    f.clientRef,
		Replacements: []remoteReplacementPayload{{ProductUID: "not-here", Barcode: "nope", Quantity: 1}},
	})
	if err := f.app.executeRemoteReturnInvoiceCommand(ctx, payload); err == nil {
		t.Fatal("expected an exchange for a product this till doesn't have to fail")
	}
}
