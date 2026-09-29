package main

import (
	"context"
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// syncCurrency / syncPaymentMethod put a currency or a payment method on the
// till the only way one can arrive now: from kashi, through the sync. The
// App methods these tests used to call are gone.
func syncCurrency(t *testing.T, app *App, code, name, symbol string, unitsPerUsdMicros, roundingUnit int64) {
	t.Helper()
	if err := applySyncedCurrency(context.Background(), app.store.Queries, syncedCurrency{
		Code: code, Name: name, Symbol: symbol, IsActive: true,
		UnitsPerUsdMicros: unitsPerUsdMicros, CashRoundingUnit: roundingUnit,
	}); err != nil {
		t.Fatal(err)
	}
}

func syncPaymentMethod(t *testing.T, app *App, name, currencyCode string) db.CashboxAccount {
	t.Helper()
	if err := applySyncedCashboxAccount(context.Background(), app.store.Queries, syncedCashboxAccount{
		Name: name, CurrencyCode: currencyCode,
	}); err != nil {
		t.Fatal(err)
	}
	account, err := app.store.GetCashboxAccountByName(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

// A currency kashi sends must round-trip its rate and be immediately usable
// for cash rounding -- it is a first-class currency, not a hardcoded
// LBP/USD special case.
func TestSyncedCurrencyRoundTripsRateAndIsUsableForCashRounding(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	// 0.92 EUR/USD expressed the way kashi sends it, in micros.
	syncCurrency(t, app, "EUR", "Euro", "\u20ac", 920000, 5)

	currency, err := app.store.GetCurrencyByCode(app.ctx, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if currency.Symbol != "\u20ac" || currency.UnitsPerUsdMicros != 920000 {
		t.Fatalf("synced currency = %#v, want symbol \u20ac and 920000 micros", currency)
	}
	if !currency.IsActive || currency.IsDefault {
		t.Fatalf("synced currency state = %#v, want active, non-default", currency)
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

// TestCheckoutAcceptsExactPaymentThroughANewlyAddedCurrency exercises the
// full generalized rounding path end to end: sync down a currency the till
// has never seen, configure it as the rounding currency, and confirm
// checkout accepts a payment computed with the same money.go functions the
// backend itself uses - i.e. the new currency is a first-class citizen, not
// a hardcoded LBP/USD special case.
func TestCheckoutAcceptsExactPaymentThroughANewlyAddedCurrency(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}

	syncCurrency(t, app, "EUR", "Euro", "€", 920000, 5)
	eurAccount := syncPaymentMethod(t, app, "Euro Cash", "EUR")
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

	shift, err := app.store.CreateShift(app.ctx, db.CreateShiftParams{})
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
	priceProductsFromRows(t, app.store.Queries)
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
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: eurAccount.ID, AmountCents: amountCents}})
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
