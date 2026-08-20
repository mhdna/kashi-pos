package main

import (
	"database/sql"
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

func setupQuoteTestApp(t *testing.T, taxRate float64, roundingMode, roundingCurrency string) (*App, db.CashboxAccount, db.Client, *sql.DB) {
	t.Helper()
	app, database := newProductTestApp(t)
	t.Cleanup(func() { database.Close() })

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', ?, ?, 0, 0)`, taxRate, roundingMode); err != nil {
		t.Fatal(err)
	}
	settings, err := app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RoundingCurrency = roundingCurrency
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}

	if _, err := app.store.CreateShift(app.ctx, 0); err != nil {
		t.Fatal(err)
	}
	cashbox, err := app.store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := app.store.CreateInventory(app.ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	settings, err = app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SalesInventoryID = inventory.ID
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}
	product, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 1533, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 100}); err != nil {
		t.Fatal(err)
	}
	return app, cashbox, client, database
}

func cartJSON(qty int) string {
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": qty}})
	return string(items)
}

// TestQuoteInvoiceTotalMatchesCheckoutGrandTotal is the core regression
// guard for "calculator total doesn't match what's saved to the invoice":
// QuoteInvoiceTotal and salesInvoiceTx must agree exactly, for a range of
// tax rates and rounding modes, including a fractional ("points") unit
// price that isn't a round number of cents-per-unit once tax is applied.
func TestQuoteInvoiceTotalMatchesCheckoutGrandTotal(t *testing.T) {
	cases := []struct {
		name             string
		taxRate          float64
		roundingMode     string
		roundingCurrency string
		qty              int
	}{
		{"no tax, half up, LBP rounding", 0, "half_up", "LBP", 1},
		{"11% tax, half up, LBP rounding", 0.11, "half_up", "LBP", 3},
		{"11% tax, round down, LBP rounding", 0.11, "down", "LBP", 3},
		{"11% tax, round up, LBP rounding", 0.11, "up", "LBP", 3},
		{"7% tax, half up, USD rounding", 0.07, "half_up", "USD", 5},
		{"fractional unit price compounding over many units", 0.11, "half_up", "LBP", 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, cashbox, client, _ := setupQuoteTestApp(t, tc.taxRate, tc.roundingMode, tc.roundingCurrency)
			items := cartJSON(tc.qty)

			quote, err := app.QuoteInvoiceTotal(items)
			if err != nil {
				t.Fatal(err)
			}

			payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
			result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: client.ID})
			if err != nil {
				t.Fatalf("checkout rejected the exact quoted total: %v", err)
			}
			if result.GrandTotal != quote.GrandTotalCents {
				t.Fatalf("checkout grand total = %d, quote predicted %d", result.GrandTotal, quote.GrandTotalCents)
			}
		})
	}
}

// TestQuotePaymentStatusPredictsCheckoutAcceptance directly reproduces the
// reported bug ("amount due 20,000, I enter 20,000, it says not paid, 0+0")
// by checking that QuotePaymentStatus's CheckoutReady flag is true if and
// only if salesInvoiceTx actually accepts the same payments.
func TestQuotePaymentStatusPredictsCheckoutAcceptance(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0.11, "half_up", "LBP")
	items := cartJSON(3)

	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}

	exactPayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
	status, err := app.QuotePaymentStatus(items, string(exactPayments))
	if err != nil {
		t.Fatal(err)
	}
	if !status.CheckoutReady || !status.IsPaidInFull {
		t.Fatalf("quote says not ready for the exact total: %+v", status)
	}
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(exactPayments), ClientID: client.ID}); err != nil {
		t.Fatalf("checkout rejected a payment the quote said was ready: %v", err)
	}

	shortPayments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents - 100}})
	shortStatus, err := app.QuotePaymentStatus(items, string(shortPayments))
	if err != nil {
		t.Fatal(err)
	}
	if shortStatus.CheckoutReady {
		t.Fatalf("quote said a genuinely short payment was ready: %+v", shortStatus)
	}
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(shortPayments), ClientID: client.ID}); err == nil {
		t.Fatal("checkout accepted a payment the quote said was not ready")
	}
}

// TestQuotePaymentStatusAcceptsLBPCashRoundedEquivalent covers paying in a
// physically-rounded currency (LBP): entering the exact LBP amount
// QuoteInvoiceTotal implies (via ConvertCashEntryToCents, mirroring what
// the calculator does) must satisfy CheckoutReady and actual checkout.
func TestQuotePaymentStatusAcceptsLBPCashRoundedEquivalent(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0.11, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	settings, err := app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RoundingCurrency = "LBP"
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.store.CreateShift(app.ctx, 0); err != nil {
		t.Fatal(err)
	}
	lbpAccount, err := app.store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "LBP Cash", CurrencyCode: "LBP"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := app.store.CreateInventory(app.ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	settings, err = app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SalesInventoryID = inventory.ID
	if err := app.store.UpdateSettings(app.ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}
	product, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 1533, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 100}); err != nil {
		t.Fatal(err)
	}

	items := cartJSON(3)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	lbpCurrency, err := app.store.GetCurrencyByCode(app.ctx, "LBP")
	if err != nil {
		t.Fatal(err)
	}
	dueLbpUnits, err := roundCurrencyUnitsFromCents(quote.GrandTotalCents, lbpCurrency.UnitsPerUsdMicros, lbpCurrency.CashRoundingUnit, quote.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}

	// This is exactly what the calculator does: convert the displayed LBP
	// amount due into cents via ConvertCashEntryToCents, then ask whether
	// that's enough.
	amountCents, err := app.ConvertCashEntryToCents("LBP", float64(dueLbpUnits))
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: lbpAccount.ID, AmountCents: amountCents}})
	status, err := app.QuotePaymentStatus(items, string(payments))
	if err != nil {
		t.Fatal(err)
	}
	if !status.CheckoutReady {
		t.Fatalf("entering the exact displayed LBP amount was not accepted as ready: %+v (dueLbpUnits=%d)", status, dueLbpUnits)
	}
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatalf("checkout rejected the exact displayed LBP amount: %v", err)
	}
}

// TestQuoteAndConvertCashEntryAgreeForANewlyAddedCurrency proves the whole
// quote/payment-status/entry-conversion path is generic, not hardcoded to
// LBP/USD, by running it against a currency created at runtime.
func TestQuoteAndConvertCashEntryAgreeForANewlyAddedCurrency(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0.11, "half_up", "USD")
	_ = cashbox
	if _, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 5); err != nil {
		t.Fatal(err)
	}
	eurAccount, err := app.CreateCashboxAccount("Euro Cash", "EUR")
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

	items := cartJSON(3)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	if quote.RoundingCurrencyCode != "EUR" {
		t.Fatalf("quote rounding currency = %q, want EUR", quote.RoundingCurrencyCode)
	}
	eurCurrency, err := app.store.GetCurrencyByCode(app.ctx, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	dueEurUnits, err := roundCurrencyUnitsFromCents(quote.GrandTotalCents, eurCurrency.UnitsPerUsdMicros, eurCurrency.CashRoundingUnit, quote.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}
	amountCents, err := app.ConvertCashEntryToCents("EUR", float64(dueEurUnits))
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: eurAccount["id"].(int64), AmountCents: amountCents}})
	status, err := app.QuotePaymentStatus(items, string(payments))
	if err != nil {
		t.Fatal(err)
	}
	if !status.CheckoutReady {
		t.Fatalf("entering the exact displayed EUR amount was not accepted as ready: %+v", status)
	}
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatalf("checkout rejected the exact displayed EUR amount: %v", err)
	}
}
