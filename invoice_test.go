package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestSalesInvoiceTransactionUsesAuthoritativeTotalsAndStock(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "invoice.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0.11, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSettings(ctx, db.UpdateSettingsParams{
		BranchName: settings.BranchName, BranchCode: settings.BranchCode,
		ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, ReceiptEnabled: 0,
		SalesInventoryID: settings.SalesInventoryID,
	}); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, 0)
	if err != nil {
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
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 123, Discount: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSettings(ctx, db.UpdateSettingsParams{
		BranchName: settings.BranchName, BranchCode: settings.BranchCode,
		ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, ReceiptEnabled: 0,
		SalesInventoryID: inventory.ID,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	year := int64(time.Now().Year())
	nextCode, err := app.GetNextInvoiceCode("sales")
	if err != nil {
		t.Fatal(err)
	}
	if expected := formatInvoiceCode("SA", "CN", year, 1); nextCode != expected {
		t.Fatalf("next sales code = %q, want %q", nextCode, expected)
	}
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 3, "price": 9999, "discount": 100}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 369}})
	result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.InvoiceCode == "" {
		t.Fatal("invoice code is empty")
	}
	if expected := formatInvoiceCode("SA", "CN", year, 1); result.InvoiceCode != expected {
		t.Fatalf("sales code = %q, want %q", result.InvoiceCode, expected)
	}
	nextCode, err = app.GetNextInvoiceCode("sales")
	if err != nil {
		t.Fatal(err)
	}
	if expected := formatInvoiceCode("SA", "CN", year, 2); nextCode != expected {
		t.Fatalf("next sales code = %q, want %q", nextCode, expected)
	}
	exchangeCode, err := app.GetNextInvoiceCode("exchange")
	if err != nil {
		t.Fatal(err)
	}
	if expected := formatInvoiceCode("EX", "CN", year, 1); exchangeCode != expected {
		t.Fatalf("next exchange code = %q, want %q", exchangeCode, expected)
	}
	invoice, err := store.GetInvoiceByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if invoice.Subtotal != 369 || invoice.DiscountedTotal != 332 || invoice.GrandTotal != 369 {
		t.Fatalf("totals = subtotal %d discounted %d grand %d", invoice.Subtotal, invoice.DiscountedTotal, invoice.GrandTotal)
	}
	stock, err := store.GetInventoryItem(ctx, db.GetInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID})
	if err != nil {
		t.Fatal(err)
	}
	if stock.Quantity != 2 {
		t.Fatalf("stock = %d, want 2", stock.Quantity)
	}
	if client, err := store.GetClientByID(ctx, client.ID); err != nil || client.TotalLoyaltyPoints != 3 {
		t.Fatalf("loyalty points = %+v, err=%v", client, err)
	}
	_ = shift
}

func TestNetPaymentsByAccountAllowsNegativeAndSkipsZero(t *testing.T) {
	payments := []checkoutPayment{
		{AccountID: 1, AmountCents: 1000, EntryType: "payment"},
		{AccountID: 2, AmountCents: -500, EntryType: "change"},
		{AccountID: 3, AmountCents: 500, EntryType: "payment"},
		{AccountID: 3, AmountCents: -500, EntryType: "change"},
	}

	net := netPaymentsByAccount(payments)
	if net[1] != 1000 || net[2] != -500 || net[3] != 0 {
		t.Fatalf("net payments = %#v, want account 1=1000, account 2=-500, account 3=0", net)
	}
}

func TestValidatePaymentsAllowsOverpaymentAndRejectsUnderpayment(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "payments.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{
		Name: "Cash", CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := validatePayments(ctx, store.Queries, []checkoutPayment{{
		AccountID: cashbox.ID, AmountCents: 1800, EntryType: "payment",
	}}, 1780, 100); err != nil {
		t.Fatalf("overpayment should be accepted: %v", err)
	}
	if err := validatePayments(ctx, store.Queries, []checkoutPayment{{
		AccountID: cashbox.ID, AmountCents: 1881, EntryType: "payment",
	}}, 1780, 100); err == nil {
		t.Fatal("overpayment above the configured limit should be rejected")
	}

	err = validatePayments(ctx, store.Queries, []checkoutPayment{{
		AccountID: cashbox.ID, AmountCents: 1779, EntryType: "payment",
	}}, 1780, 100)
	if err == nil || !strings.Contains(err.Error(), "less than invoice total") {
		t.Fatalf("underpayment error = %v, want less-than-total error", err)
	}
}
