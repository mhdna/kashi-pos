package main

import (
	"context"
	"testing"

	db "kashi-pos/db/sqlc"
)

// addPaymentMethod puts a payment method in the database the way the sync
// does. Tests used to call App.CreateCashboxAccount for this, but a till has
// no such method any more -- payment methods come from kashi.
func addPaymentMethod(t *testing.T, app *App, name, currencyCode string) db.CashboxAccount {
	t.Helper()
	account, err := app.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
		Name:         name,
		CurrencyCode: currencyCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	return account
}

// paymentMethodNames extracts the "name" field, in order, from a
// ListPaymentMethods result for easy comparison in assertions.
func paymentMethodNames(t *testing.T, methods []map[string]any) []string {
	t.Helper()
	names := make([]string, len(methods))
	for i, m := range methods {
		name, ok := m["name"].(string)
		if !ok {
			t.Fatalf("payment method %#v has no string name", m)
		}
		names[i] = name
	}
	return names
}

func assertNamesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("payment method order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("payment method order = %v, want %v", got, want)
		}
	}
}

func TestListPaymentMethodsOrdersBySortOrderWithSpecialsPinnedLast(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	addPaymentMethod(t, app, "Cash USD", "USD")
	addPaymentMethod(t, app, "Visa", "USD")
	addPaymentMethod(t, app, "Whish", "USD")

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	// Created in this order, and On Account/On Coupon self-heal in only when
	// ListPaymentMethods first runs - so they always land after every real
	// method regardless of when they were actually inserted.
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{
		"Cash USD", "Visa", "Whish", "On Account", "On Coupon",
	})
}

func TestListPaymentMethodsShowsTheStoredNameWithNoPositionalOverride(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	addPaymentMethod(t, app, "Cash USD", "USD")
	addPaymentMethod(t, app, "Cash LBP", "LBP")

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range methods {
		if m["name"] == "Cash USD" || m["name"] == "Cash LBP" {
			if m["displayName"] != m["name"] {
				t.Fatalf("displayName = %v, want it to match the stored name %v", m["displayName"], m["name"])
			}
		}
	}
}

func TestHidePaymentMethodCurrencyDefaultsToVisibleAndPersistsWhenToggled(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}

	settings, err := app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["hidePaymentMethodCurrency"] != false {
		t.Fatalf("default hidePaymentMethodCurrency = %v, want false", settings["hidePaymentMethodCurrency"])
	}

	if err := app.SetHidePaymentMethodCurrency(true); err != nil {
		t.Fatal(err)
	}
	settings, err = app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["hidePaymentMethodCurrency"] != true {
		t.Fatalf("hidePaymentMethodCurrency after enabling = %v, want true", settings["hidePaymentMethodCurrency"])
	}

	if err := app.SetHidePaymentMethodCurrency(false); err != nil {
		t.Fatal(err)
	}
	settings, err = app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["hidePaymentMethodCurrency"] != false {
		t.Fatalf("hidePaymentMethodCurrency after disabling = %v, want false", settings["hidePaymentMethodCurrency"])
	}
}

// TestPaymentMethodColorFlowsIntoShiftBalancesAndIncome confirms a payment
// method's color reaches the same per-account balance breakdowns the shift
// closing/report screens show, not just the checkout calculator - both
// ListShiftBalances and GetCurrentShiftIncomeByAccount select through
// cashbox_accounts.color. The color is kashi's now and arrives via
// applySyncedCashboxAccount, so the sync's own write is what is exercised.
func TestPaymentMethodColorFlowsIntoShiftBalancesAndIncome(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)

	updated, err := fixture.app.store.UpdateCashboxAccount(context.Background(), db.UpdateCashboxAccountParams{
		ID: fixture.sourceID, Name: "Cash", CurrencyCode: "USD", Color: "#2E7D32",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Color != "#2E7D32" {
		t.Fatalf("UpdateCashboxAccount color = %v, want #2E7D32", updated.Color)
	}

	balances, err := fixture.app.ListShiftBalances(fixture.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	foundBalance := false
	for _, row := range balances {
		if row["accountId"] == fixture.sourceID {
			foundBalance = true
			if row["color"] != "#2E7D32" {
				t.Fatalf("ListShiftBalances color = %v, want #2E7D32", row["color"])
			}
		}
	}
	if !foundBalance {
		t.Fatal("source account not found in ListShiftBalances")
	}

	income, err := fixture.app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	foundIncome := false
	for _, row := range income {
		if row["accountId"] == fixture.sourceID {
			foundIncome = true
			if row["color"] != "#2E7D32" {
				t.Fatalf("GetCurrentShiftIncomeByAccount color = %v, want #2E7D32", row["color"])
			}
		}
	}
	if !foundIncome {
		t.Fatal("source account not found in GetCurrentShiftIncomeByAccount")
	}
}
