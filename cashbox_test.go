package main

import "testing"

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

	if _, err := app.CreateCashboxAccount("Cash USD", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCashboxAccount("Visa", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCashboxAccount("Whish", "USD"); err != nil {
		t.Fatal(err)
	}

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

	if _, err := app.CreateCashboxAccount("Cash USD", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCashboxAccount("Cash LBP", "LBP"); err != nil {
		t.Fatal(err)
	}

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

func TestCreateCashboxAccountAppendsToTheEndOfTheOrder(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := app.CreateCashboxAccount("Cash USD", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateCashboxAccount("Visa", "USD"); err != nil {
		t.Fatal(err)
	}

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{"Cash USD", "Visa", "On Account", "On Coupon"})

	if _, err := app.CreateCashboxAccount("Whish", "USD"); err != nil {
		t.Fatal(err)
	}
	methods, err = app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{"Cash USD", "Visa", "Whish", "On Account", "On Coupon"})
}

func TestReorderCashboxAccountsAppliesTheNewOrderAndPersists(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	usd, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}
	visa, err := app.CreateCashboxAccount("Visa", "USD")
	if err != nil {
		t.Fatal(err)
	}
	lbp, err := app.CreateCashboxAccount("Cash LBP", "LBP")
	if err != nil {
		t.Fatal(err)
	}

	// Move Cash LBP to the front.
	newOrder := []int64{lbp["id"].(int64), usd["id"].(int64), visa["id"].(int64)}
	if err := app.ReorderCashboxAccounts(newOrder); err != nil {
		t.Fatal(err)
	}

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{"Cash LBP", "Cash USD", "Visa", "On Account", "On Coupon"})
}

func TestReorderCashboxAccountsRejectsAMismatchedSet(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	usd, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}
	visa, err := app.CreateCashboxAccount("Visa", "USD")
	if err != nil {
		t.Fatal(err)
	}
	// Force On Account/On Coupon to exist so they can be tested as rejected.
	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	var onAccountID int64
	for _, m := range methods {
		if m["name"] == "On Account" {
			onAccountID = m["id"].(int64)
		}
	}

	cases := map[string][]int64{
		"too few":            {usd["id"].(int64)},
		"unknown id":         {usd["id"].(int64), visa["id"].(int64) + 1000},
		"duplicate id":       {usd["id"].(int64), usd["id"].(int64)},
		"includes a special": {usd["id"].(int64), onAccountID},
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			if err := app.ReorderCashboxAccounts(ids); err == nil {
				t.Fatalf("expected an error for %s (%v)", name, ids)
			}
		})
	}

	// Confirm a rejected reorder made no partial changes.
	methods, err = app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{"Cash USD", "Visa", "On Account", "On Coupon"})
}

func TestUpdateCashboxAccountRenamesAndChangesCurrency(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 1); err != nil {
		t.Fatal(err)
	}
	account, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := app.UpdateCashboxAccount(account["id"].(int64), "Euro Cash", "EUR", "")
	if err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Euro Cash" || updated["currencyCode"] != "EUR" {
		t.Fatalf("updated account = %#v, want name Euro Cash, currency EUR", updated)
	}

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	assertNamesEqual(t, paymentMethodNames(t, methods), []string{"Euro Cash", "On Account", "On Coupon"})
}

func TestUpdateCashboxAccountRejectsUnknownOrInactiveCurrency(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	account, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.UpdateCashboxAccount(account["id"].(int64), "Cash USD", "EUR", ""); err == nil {
		t.Fatal("expected error for a currency that does not exist")
	}
	if _, err := app.CreateCurrency("EUR", "Euro", "€", 0.92, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetCurrencyActive("EUR", false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UpdateCashboxAccount(account["id"].(int64), "Cash USD", "EUR", ""); err == nil {
		t.Fatal("expected error updating to a deactivated currency")
	}
}

func TestUpdateCashboxAccountRejectsEditingReservedAccounts(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	var onAccountID int64
	for _, m := range methods {
		if m["name"] == "On Account" {
			onAccountID = m["id"].(int64)
		}
	}
	if onAccountID == 0 {
		t.Fatal("expected On Account to have been self-healed by ListPaymentMethods")
	}

	if _, err := app.UpdateCashboxAccount(onAccountID, "Renamed", "USD", ""); err == nil {
		t.Fatal("expected error editing the reserved On Account payment method")
	}

	account, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.UpdateCashboxAccount(account["id"].(int64), "On Coupon", "USD", ""); err == nil {
		t.Fatal("expected error renaming a normal payment method to a reserved name")
	}
}

func TestUpdateCashboxAccountSetsAndClearsColor(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	account, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}
	if account["color"] != nil && account["color"] != "" {
		t.Fatalf("newly created account color = %v, want empty", account["color"])
	}

	updated, err := app.UpdateCashboxAccount(account["id"].(int64), "Cash USD", "USD", "#2E7D32")
	if err != nil {
		t.Fatal(err)
	}
	if updated["color"] != "#2E7D32" {
		t.Fatalf("updated color = %v, want #2E7D32", updated["color"])
	}

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range methods {
		if m["name"] == "Cash USD" {
			found = true
			if m["color"] != "#2E7D32" {
				t.Fatalf("ListPaymentMethods color = %v, want #2E7D32", m["color"])
			}
		}
	}
	if !found {
		t.Fatal("Cash USD not found in ListPaymentMethods")
	}

	// Clearing the color back to "" is valid - it means "use the automatic
	// name-based color" rather than being rejected as invalid input.
	cleared, err := app.UpdateCashboxAccount(account["id"].(int64), "Cash USD", "USD", "")
	if err != nil {
		t.Fatal(err)
	}
	if cleared["color"] != "" {
		t.Fatalf("cleared color = %v, want empty", cleared["color"])
	}
}

func TestUpdateCashboxAccountRejectsMalformedColor(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	account, err := app.CreateCashboxAccount("Cash USD", "USD")
	if err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{"green", "#fff", "#gggggg", "2E7D32", "#2E7D3"} {
		if _, err := app.UpdateCashboxAccount(account["id"].(int64), "Cash USD", "USD", bad); err == nil {
			t.Fatalf("expected error for malformed color %q", bad)
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

// TestPaymentMethodColorFlowsIntoShiftBalancesAndIncome confirms an admin's
// custom color for a payment method (settable in Settings) reaches the same
// per-account balance breakdowns the shift closing/report screens show, not
// just the checkout calculator - both ListShiftBalances and
// GetCurrentShiftIncomeByAccount select through cashbox_accounts.color now.
func TestPaymentMethodColorFlowsIntoShiftBalancesAndIncome(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)

	updated, err := fixture.app.UpdateCashboxAccount(fixture.sourceID, "Cash", "USD", "#2E7D32")
	if err != nil {
		t.Fatal(err)
	}
	if updated["color"] != "#2E7D32" {
		t.Fatalf("UpdateCashboxAccount color = %v, want #2E7D32", updated["color"])
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
