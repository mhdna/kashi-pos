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
		TaxRate: settings.TaxRate, RoundingMode: settings.RoundingMode, RoundingCurrency: roundingCurrencyUSD, ReceiptEnabled: settings.ReceiptEnabled,
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

func TestValidateExchangeWindow(t *testing.T) {
	createdAt := "2026-07-26 12:00:00"
	if err := validateExchangeWindow(createdAt, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("exchange at exactly 3 days should be allowed: %v", err)
	}
	if err := validateExchangeWindow(createdAt, time.Date(2026, 7, 29, 12, 0, 1, 0, time.UTC)); err == nil {
		t.Fatal("exchange after 3 days should be rejected")
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

func TestParseExchangeRequestRejectsReturnedProductAsReplacement(t *testing.T) {
	if _, err := parseExchangeRequest(`[
		{"code":"PRD-2","qty":-1},
		{"code":"prd-2","qty":1},
		{"code":"OTHER","qty":1}
	]`); err == nil {
		t.Fatal("expected returned product used as a replacement to fail")
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
