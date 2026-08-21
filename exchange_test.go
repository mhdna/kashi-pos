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
	salesperson, err := store.CreateSalesperson(ctx, "Exchange Salesperson")
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
		Barcode: "222", Code: "REPLACE-1", Name: "First replacement", IsActive: 1, Price: 600,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondReplacement, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "333", Code: "REPLACE-2", Name: "Second replacement", IsActive: 1, Price: 600,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range []db.Product{returnedProduct, replacementProduct, secondReplacement} {
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
	loadedClient, ok := loaded["client"].(map[string]any)
	if !ok || loadedClient["id"] != client.ID {
		t.Fatalf("loaded client = %#v", loaded["client"])
	}

	sourceID := loaded["id"].(int64)
	tooLowItems, _ := json.Marshal([]exchangeLine{
		{Code: returnedProduct.Code, Qty: -1},
		{Code: replacementProduct.Code, Qty: 1},
	})
	if _, err := app.CheckoutExchangeInvoice(string(tooLowItems), "[]", sourceID, 0, salesperson.ID, ""); err == nil {
		t.Fatal("expected a replacement total below the return value to fail")
	}

	exchangeItems, _ := json.Marshal([]exchangeLine{
		{Code: returnedProduct.Code, Qty: -1},
		{Code: replacementProduct.Code, Qty: 1},
		{Code: secondReplacement.Code, Qty: 1},
	})
	exchangePayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 200}})
	exchangeResult, err := app.CheckoutExchangeInvoice(string(exchangeItems), string(exchangePayments), sourceID, 0, salesperson.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	year := int64(time.Now().Year())
	if expected := formatInvoiceCode("EX", "CN", year, 1); exchangeResult["invoiceCode"] != expected {
		t.Fatalf("exchange code = %q, want %q", exchangeResult["invoiceCode"], expected)
	}
	nextExchangeCode, err := app.GetNextInvoiceCode("exchange")
	if err != nil {
		t.Fatal(err)
	}
	if expected := formatInvoiceCode("EX", "CN", year, 2); nextExchangeCode != expected {
		t.Fatalf("next exchange code = %q, want %q", nextExchangeCode, expected)
	}
	exchangeInvoice, err := store.GetInvoiceByID(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	exchangeLines, err := store.ListInvoiceProducts(ctx, exchangeInvoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exchangeInvoice.GrandTotal != 200 || len(exchangeLines) != 3 {
		t.Fatalf("exchange total=%d lines=%d", exchangeInvoice.GrandTotal, len(exchangeLines))
	}
	if exchangeInvoice.SalespersonID != salesperson.ID {
		t.Fatalf("exchange salesperson=%d, want %d", exchangeInvoice.SalespersonID, salesperson.ID)
	}
	exchangeDetails, err := app.GetInvoiceDetails(exchangeInvoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exchangeDetails["salespersonName"] != salesperson.FullName {
		t.Fatalf("exchange salesperson name=%q, want %q", exchangeDetails["salespersonName"], salesperson.FullName)
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
	secondReplacementStock, err := store.GetInventoryItem(ctx, db.GetInventoryItemParams{
		InventoryID: inventory.ID, ProductID: secondReplacement.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if returnedStock.Quantity != 5 || replacementStock.Quantity != 5 || secondReplacementStock.Quantity != 5 {
		t.Fatalf(
			"stock after exchange: returned=%d first=%d second=%d",
			returnedStock.Quantity,
			replacementStock.Quantity,
			secondReplacementStock.Quantity,
		)
	}

	if _, err := app.GetExchangeInvoice(sale.InvoiceCode); err == nil {
		t.Fatal("expected fully returned invoice to have no available items")
	}
	if _, err := app.CheckoutExchangeInvoice(string(exchangeItems), string(exchangePayments), sourceID, 0, salesperson.ID, ""); err == nil {
		t.Fatal("expected duplicate exchange to fail")
	}

	if _, err := database.Exec(
		`UPDATE invoices SET created_at = datetime('now', '-4 days') WHERE id = ?`,
		sourceID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetExchangeInvoice(sale.InvoiceCode); err == nil || !strings.Contains(err.Error(), "more than 3 days") {
		t.Fatalf("expired invoice load error = %v", err)
	}
	if _, err := app.CheckoutExchangeInvoice(string(exchangeItems), string(exchangePayments), sourceID, 0, salesperson.ID, ""); err == nil || !strings.Contains(err.Error(), "more than 3 days") {
		t.Fatalf("expired invoice checkout error = %v", err)
	}
}

func TestCreateExchangeInvoicePersistsSalesperson(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "exchange-salesperson.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, 0)
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
	salesperson, err := store.CreateSalesperson(ctx, "Exchange Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	year := int64(time.Now().Year())
	source, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: "INV-SOURCE",
		InvoiceIndex: 1, Year: year, ClientID: client.ID, InventoryID: inventory.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSalesInvoice(ctx, source.ID); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	exchange, err := createExchangeInvoice(
		app, store.Queries, cashbox, shift, source, inventory.ID, salesperson.ID, 200, "CN", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if exchange.SalespersonID != salesperson.ID {
		t.Fatalf("exchange salesperson=%d, want %d", exchange.SalespersonID, salesperson.ID)
	}
	details, err := app.GetInvoiceDetails(exchange.ID)
	if err != nil {
		t.Fatal(err)
	}
	if details["salespersonName"] != salesperson.FullName {
		t.Fatalf("exchange salesperson name=%q, want %q", details["salespersonName"], salesperson.FullName)
	}
}

// newExchangeReplacementPricingFixture sets up a sold-and-returnable product
// plus one replacement product, ready to check out an exchange against.
func newExchangeReplacementPricingFixture(t *testing.T) (app *App, sourceID int64, salesperson db.Salesperson, replacementProduct db.Product, cashbox db.CashboxAccount) {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "exchange-pricing.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateShift(ctx, 0); err != nil {
		t.Fatal(err)
	}
	cashbox, err = store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
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
	salesperson, err = store.CreateSalesperson(ctx, "Pricing Salesperson")
	if err != nil {
		t.Fatal(err)
	}
	returnedProduct, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "R1", Code: "RETURN-P", Name: "Returned product", IsActive: 1, Price: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	replacementProduct, err = store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "P1", Code: "REPLACE-P", Name: "Replacement product", IsActive: 1, Price: 500,
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
		BranchName: settings.BranchName, BranchCode: settings.BranchCode,
		ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme,
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, RoundingCurrency: "USD", ReceiptEnabled: settings.ReceiptEnabled,
		SalesInventoryID: settings.SalesInventoryID, ExchangeWindowHours: settings.ExchangeWindowHours,
	}); err != nil {
		t.Fatal(err)
	}

	app = &App{ctx: ctx, store: store}
	saleItems, _ := json.Marshal([]salesInvoiceItem{{Code: returnedProduct.Code, Qty: 3}})
	salePayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 3000}})
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
	return app, loaded["id"].(int64), salesperson, replacementProduct, cashbox
}

func TestExchangeReplacementRejectsCustomPriceWhenFlagDisabled(t *testing.T) {
	app, sourceID, salesperson, replacementProduct, _ := newExchangeReplacementPricingFixture(t)

	customPrice := int64(1500)
	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: replacementProduct.Code, Qty: 1, CustomPriceCents: &customPrice},
	})
	if _, err := app.CheckoutExchangeInvoice(string(items), "[]", sourceID, 0, salesperson.ID, ""); err == nil || !strings.Contains(err.Error(), "custom item pricing is disabled") {
		t.Fatalf("custom price on a replacement with the flag disabled = %v, want a disabled-flag error", err)
	}
}

func TestExchangeReplacementSupportsCustomPriceAndPerUnitDiscount(t *testing.T) {
	app, sourceID, salesperson, replacementProduct, _ := newExchangeReplacementPricingFixture(t)

	if err := app.store.SetFeatureFlag(app.ctx, db.SetFeatureFlagParams{Key: customItemPricesFlag, Enabled: 1}); err != nil {
		t.Fatal(err)
	}
	if err := app.store.SetFeatureFlag(app.ctx, db.SetFeatureFlagParams{Key: perUnitItemPricesFlag, Enabled: 1}); err != nil {
		t.Fatal(err)
	}

	// A custom-priced replacement (product catalog price 500, custom 2000)
	// with per-unit discounts on its two units (0% and 50%) should price as
	// 2000 + (2000 - 1000) = 3000, not the catalog default.
	customPrice := int64(2000)
	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -3},
		{
			Code: replacementProduct.Code, Qty: 2, CustomPriceCents: &customPrice,
			UnitPricesCents: []int64{2000, 2000}, UnitDiscounts: []int64{0, 50},
		},
	})
	// Returned total: 3 * 1000 = 3000. Replacement total: 2000 + 1000 = 3000.
	// Net difference is 0, so no payment is required.
	result, err := app.CheckoutExchangeInvoice(string(items), "[]", sourceID, 0, salesperson.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	invoiceCode, ok := result["invoiceCode"].(string)
	if !ok || invoiceCode == "" {
		t.Fatalf("exchange result = %#v, want an invoiceCode", result)
	}
	// The fixture's sale is invoice 1; this exchange is the second invoice
	// created in the fresh database.
	invoice, err := app.store.GetInvoiceByID(app.ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if invoice.GrandTotal != 0 {
		t.Fatalf("exchange grand total = %d, want 0 (returned and custom-priced replacement totals match)", invoice.GrandTotal)
	}
	lines, err := app.store.ListInvoiceProducts(app.ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	var replacementLines int
	for _, line := range lines {
		if line.Quantity <= 0 {
			continue
		}
		replacementLines++
		if line.UnitPrice != 2000 {
			t.Fatalf("replacement line unit price = %d, want the custom 2000", line.UnitPrice)
		}
	}
	if replacementLines != 2 {
		t.Fatalf("replacement lines = %d, want 2 (one per distinct per-unit discount bucket)", replacementLines)
	}
}

func TestValidateExchangeWindow(t *testing.T) {
	createdAt := "2026-07-26 12:00:00"
	window := exchangeWindowDuration(72)
	if err := validateExchangeWindow(createdAt, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC), window); err != nil {
		t.Fatalf("exchange at exactly 3 days should be allowed: %v", err)
	}
	if err := validateExchangeWindow(createdAt, time.Date(2026, 7, 29, 12, 0, 1, 0, time.UTC), window); err == nil {
		t.Fatal("exchange after 3 days should be rejected")
	}
}

func TestValidateExchangeWindowZeroMeansUnlimited(t *testing.T) {
	createdAt := "2020-01-01 00:00:00"
	if err := validateExchangeWindow(createdAt, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), exchangeWindowDuration(0)); err != nil {
		t.Fatalf("a zero exchange window should mean no time limit: %v", err)
	}
}

func TestParseExchangeRequestMergesDuplicateReplacementLines(t *testing.T) {
	request, err := parseExchangeRequest(`[
		{"code":"RETURN","qty":-1},
		{"code":"REPLACE","qty":1},
		{"code":"REPLACE","qty":2}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.replacements) != 1 || request.replacements[0].Qty != 3 {
		t.Fatalf("replacements = %#v", request.replacements)
	}
}

func TestParseExchangeRequestAllowsMultipleReturnedLines(t *testing.T) {
	request, err := parseExchangeRequest(`[
		{"code":"RETURN-1","qty":-1},
		{"code":"RETURN-2","qty":-2},
		{"code":"REPLACE","qty":3}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.returnedItems) != 2 || request.returnedItems[0].Qty != -1 || request.returnedItems[1].Qty != -2 {
		t.Fatalf("returned items = %#v", request.returnedItems)
	}
}

func TestParseExchangeRequestAllowsSameProductAsReplacement(t *testing.T) {
	// Swapping a defective unit for a fresh one of the same product: the
	// returned line stays at qty -1 and the replacement stays at its own
	// qty 1, rather than netting to nothing.
	request, err := parseExchangeRequest(`[
		{"code":"PRD-2","qty":-1},
		{"code":"prd-2","qty":1}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.returnedItems) != 1 || request.returnedItems[0].Qty != -1 {
		t.Fatalf("returned items = %#v, want one line at qty -1", request.returnedItems)
	}
	if len(request.replacements) != 1 || request.replacements[0].Qty != 1 {
		t.Fatalf("replacements = %#v, want one line at qty 1", request.replacements)
	}
}

func TestAdminReturnRequestAllowsReturnOnly(t *testing.T) {
	request, err := parseExchangeRequest(`[{"code":"RETURN","qty":-1}]`, true)
	if err != nil {
		t.Fatal(err)
	}
	if request.returned.Code != "RETURN" || len(request.replacements) != 0 {
		t.Fatalf("return request = %#v", request)
	}
}

func TestSignedAdjustmentTax(t *testing.T) {
	positive, err := signedAdjustmentTax(1200, 0.1, roundingHalfUp)
	if err != nil || positive != 120 {
		t.Fatalf("positive tax=%d err=%v", positive, err)
	}
	negative, err := signedAdjustmentTax(-1200, 0.1, roundingHalfUp)
	if err != nil || negative != -120 {
		t.Fatalf("negative tax=%d err=%v", negative, err)
	}
}

func TestReturnInvoiceUsesReturnPrefix(t *testing.T) {
	typeName, prefix, prefixErr := invoiceNumberingType("return")
	if prefixErr != nil || typeName != invoiceTypeExchange || prefix != "RS" {
		t.Fatalf("return numbering type=%q prefix=%q err=%v", typeName, prefix, prefixErr)
	}
	_, prefix, prefixErr = invoiceNumberingType("exchange")
	if prefixErr != nil || prefix != "EX" {
		t.Fatalf("exchange prefix=%q err=%v", prefix, prefixErr)
	}
}

func TestQuoteExchangeTotalPricesLiveCart(t *testing.T) {
	app, sourceID, _, replacementProduct, _ := newExchangeReplacementPricingFixture(t)

	// Only a returned item selected so far, no replacement yet: nothing is
	// owed yet, but the quote must still succeed (not error out the way
	// QuoteInvoiceTotal does on a negative-qty item) and flag that more
	// replacement value is needed.
	returnOnlyItems, _ := json.Marshal([]exchangeLine{{Code: "RETURN-P", Qty: -1}})
	quote, err := app.QuoteExchangeTotal(string(returnOnlyItems), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if quote.ReturnedTotalCents != 1000 || quote.GrandTotalCents != 0 || !quote.NeedsMoreReplacementValue {
		t.Fatalf("return-only quote = %#v, want returned=1000 grandTotal=0 needsMore=true", quote)
	}

	// Add a replacement (500/unit) covering the returned value (1000) plus a
	// bit more.
	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: replacementProduct.Code, Qty: 3},
	})
	quote, err = app.QuoteExchangeTotal(string(items), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if quote.ReplacementTotalCents != 1500 || quote.GrandTotalCents != 500 || quote.NeedsMoreReplacementValue {
		t.Fatalf("covered quote = %#v, want replacement=1500 grandTotal=500 needsMore=false", quote)
	}
}

func TestQuoteExchangePaymentStatusMatchesCheckout(t *testing.T) {
	app, sourceID, _, replacementProduct, cashbox := newExchangeReplacementPricingFixture(t)

	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: replacementProduct.Code, Qty: 3},
	})
	// Underpaid: not ready.
	underpaid, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100, EntryType: "payment"}})
	status, err := app.QuoteExchangePaymentStatus(string(items), string(underpaid), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if status.GrandTotalCents != 500 || status.CheckoutReady {
		t.Fatalf("underpaid status = %#v, want grandTotal=500 checkoutReady=false", status)
	}

	// Exact payment, but not enough replacement value selected yet:
	// checkoutReady must stay false because the value gate is separate from
	// the payment gate.
	returnOnlyItems, _ := json.Marshal([]exchangeLine{{Code: "RETURN-P", Qty: -1}})
	zeroDuePayment, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: -0, EntryType: "payment"}})
	if _, err := app.QuoteExchangePaymentStatus(string(returnOnlyItems), string(zeroDuePayment), sourceID); err == nil {
		t.Fatal("expected a zero-amount payment entry to be rejected by parsePayments")
	}

	// Fully paid and enough replacement value: ready.
	paid, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 500, EntryType: "payment"}})
	status, err = app.QuoteExchangePaymentStatus(string(items), string(paid), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.CheckoutReady || !status.IsPaidInFull {
		t.Fatalf("fully paid status = %#v, want checkoutReady=true isPaidInFull=true", status)
	}
}

func TestQuoteExchangePaymentStatusReadyWithNoEntriesWhenNetZero(t *testing.T) {
	// A like-for-like exchange (returned value exactly matches replacement
	// value) needs zero payments - the calculator's entries list stays
	// empty, so an empty ("[]") payments array must still be accepted and
	// evaluated as fully paid, not rejected the way a real checkout would
	// reject a truly missing payment.
	app, sourceID, _, replacementProduct, _ := newExchangeReplacementPricingFixture(t)

	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: replacementProduct.Code, Qty: 2},
	})
	status, err := app.QuoteExchangePaymentStatus(string(items), "[]", sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if status.GrandTotalCents != 0 || status.PaidCents != 0 || !status.IsPaidInFull || !status.CheckoutReady {
		t.Fatalf("net-zero exchange status = %#v, want grandTotal=0 paid=0 isPaidInFull=true checkoutReady=true", status)
	}
}

func TestCheckoutExchangeInvoiceAllowsSameProductAsReplacement(t *testing.T) {
	// Returning a product and replacing it with a fresh unit of the exact
	// same product, at the same price/discount, must persist as two
	// distinct invoice_products rows (one negative qty, one positive) - a
	// composite (invoice_id, product_id, unit_price, discount) primary key
	// would collide here, which is exactly why invoice_products now uses a
	// surrogate id instead (see migration 000061).
	app, sourceID, salesperson, _, _ := newExchangeReplacementPricingFixture(t)

	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: "RETURN-P", Qty: 1},
	})
	result, err := app.CheckoutExchangeInvoice(string(items), "[]", sourceID, 0, salesperson.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	invoiceCode, ok := result["invoiceCode"].(string)
	if !ok || invoiceCode == "" {
		t.Fatalf("exchange result = %#v, want an invoiceCode", result)
	}
	// The fixture's sale is invoice 1; this exchange is the second invoice
	// created in the fresh database.
	lines, err := app.store.ListInvoiceProducts(app.ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("exchange lines = %d, want 2 (one returned, one replacement)", len(lines))
	}
	var sawReturned, sawReplacement bool
	for _, line := range lines {
		switch line.Quantity {
		case -1:
			sawReturned = true
		case 1:
			sawReplacement = true
		}
	}
	if !sawReturned || !sawReplacement {
		t.Fatalf("exchange lines = %#v, want one qty=-1 and one qty=1", lines)
	}
}
