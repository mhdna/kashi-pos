package main

import (
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestCreateCurrencyRoundTripsRateAndIsUsableForCashRounding(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	created, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 5)
	if err != nil {
		t.Fatal(err)
	}
	if created["code"] != "EUR" || created["symbol"] != "€" {
		t.Fatalf("created currency = %#v, want code EUR symbol €", created)
	}
	if rate, ok := created["unitsPerUsd"].(float64); !ok || rate < 0.9199 || rate > 0.9201 {
		t.Fatalf("stored rate round-trips to %v, want ~0.92", created["unitsPerUsd"])
	}

	currency, err := app.store.GetCurrencyByCode(app.ctx, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if !currency.IsActive || currency.IsDefault {
		t.Fatalf("new currency state = %#v, want active, non-default", currency)
	}

	// $10.00 (1000 cents) at 0.92 EUR/USD, rounded to the nearest 5 EUR:
	// 1000 * 920000 / (100*1_000_000) = 9.2, rounds to 10 under half_up
	// with a rounding unit of 5.
	got, err := roundCurrencyUnitsFromCents(1000, currency.UnitsPerUsdMicros, currency.CashRoundingUnit, roundingHalfUp)
	if err != nil {
		t.Fatal(err)
	}
	if got != 10 {
		t.Fatalf("roundCurrencyUnitsFromCents(1000, EUR) = %d, want 10", got)
	}
}

func TestCreateCashboxAccountRejectsUnknownOrInactiveCurrency(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := app.CreateCashboxAccount("Euro Cash", "EUR"); err == nil {
		t.Fatal("expected error for a currency that does not exist yet")
	}
	if _, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 1); err != nil {
		t.Fatal(err)
	}
	account, err := app.CreateCashboxAccount("Euro Cash", "EUR")
	if err != nil {
		t.Fatalf("expected active currency to be usable: %v", err)
	}
	if account["currencyCode"] != "EUR" {
		t.Fatalf("account currency = %v, want EUR", account["currencyCode"])
	}

	if _, err := app.SetCurrencyActive("EUR", false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCashboxAccount("Another Euro Cash", "EUR"); err == nil {
		t.Fatal("expected error creating a payment method under a deactivated currency")
	}
}

// TestCheckoutAcceptsExactPaymentThroughANewlyAddedCurrency exercises the
// full generalized rounding path end to end: create a currency the app has
// never seen before, configure it as the rounding currency, and confirm
// checkout accepts a payment computed with the same money.go functions the
// backend itself uses - i.e. the new currency is a first-class citizen, not
// a hardcoded LBP/USD special case.
func TestCheckoutAcceptsExactPaymentThroughANewlyAddedCurrency(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}

	if _, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 5); err != nil {
		t.Fatal(err)
	}
	eurAccount, err := app.CreateCashboxAccount("Euro Cash", "EUR")
	if err != nil {
		t.Fatal(err)
	}
	eurCurrency, err := app.store.GetCurrencyByCode(app.ctx, "EUR")
	if err != nil {
		t.Fatal(err)
	}

	settings, err := app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RoundingCurrency = "EUR"
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}

	shift, err := app.store.CreateShift(app.ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = shift
	inventory, err := app.store.CreateInventory(app.ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 1000, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	settings.SalesInventoryID = inventory.ID
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}

	// One unit at $10.00 (1000 cents), no discount/tax -> grand total is
	// 1000 cents rounded to EUR's nearest-5 cash unit, then converted back.
	grandTotalCents, err := roundCentsByCurrency(1000, eurCurrency.UnitsPerUsdMicros, eurCurrency.CashRoundingUnit, settings.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}
	amountCents, err := roundCashEquivalentCents(grandTotalCents, eurCurrency.UnitsPerUsdMicros, eurCurrency.CashRoundingUnit, settings.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: eurAccount["id"].(int64), AmountCents: amountCents}})
	result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatalf("checkout rejected an exact EUR payment: %v", err)
	}
	if result.GrandTotal != grandTotalCents {
		t.Fatalf("checkout grand total = %d, want %d", result.GrandTotal, grandTotalCents)
	}
}

func settingsToUpdateParams(s db.Setting) db.UpdateSettingsParams {
	return db.UpdateSettingsParams{
		BranchName: s.BranchName, BranchCode: s.BranchCode, ApiKey: s.ApiKey, PrinterID: s.PrinterID,
		PrinterSize: s.PrinterSize, ReceiptPrinter: s.ReceiptPrinter, ScreenPort: s.ScreenPort,
		ReceiptWidth: s.ReceiptWidth, ReceiptHeight: s.ReceiptHeight, Theme: s.Theme, TaxRate: s.TaxRate,
		RoundingMode: s.RoundingMode, RoundingCurrency: s.RoundingCurrency, ReceiptEnabled: s.ReceiptEnabled,
		SalesInventoryID: s.SalesInventoryID, ExchangeRate: s.ExchangeRate, MaxOverpaymentCents: s.MaxOverpaymentCents, ExchangeWindowHours: s.ExchangeWindowHours,
		MarketName: s.MarketName, MarketPhone: s.MarketPhone, BranchPhone: s.BranchPhone, MarketDescription: s.MarketDescription,
		ReturnPolicy: s.ReturnPolicy, Website: s.Website, Instagram: s.Instagram, SocialPlatforms: s.SocialPlatforms,
		SocialHandles: s.SocialHandles, ReceiptFont: s.ReceiptFont, ReceiptTitleSize: s.ReceiptTitleSize,
		ReceiptBodySize: s.ReceiptBodySize, ReceiptCutoff: s.ReceiptCutoff, ReceiptBodyFont: s.ReceiptBodyFont,
		CounterNumber: s.CounterNumber,
	}
}
