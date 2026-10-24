package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// The count_opening_cash_on_close flag decides whether the opening float is
// folded into the close-shift expectation and shown on the printed
// reconciliation. Default on (historical behaviour); off means the cashier
// counts only the shift's own cash.
func TestCloseShiftOpeningFloatCountToggle(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}

	shift, err := store.CreateShift(ctx, db.CreateShiftParams{
		OpeningFloatUsd: 5000,
		OpeningFloatLbp: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "LBP", CurrencyCode: "LBP"}); err != nil {
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
	salesperson, err := store.CreateSalesperson(ctx, "Cashier")
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/FLOAT",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 10000, DiscountedTotal: 10000, Subtotal: 10000, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 10000,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: database}
	closedAt := time.Now().Format("2006-01-02 15:04:05")

	usdBalance := func(t *testing.T, rows []map[string]any, amountKey string) int64 {
		t.Helper()
		for _, row := range rows {
			if name, _ := row["accountName"].(string); name == "USD" {
				switch v := row[amountKey].(type) {
				case float64:
					return int64(v)
				default:
					return databaseInt64(v)
				}
			}
		}
		t.Fatalf("no USD row in %#v", rows)
		return 0
	}

	// Default: flag unset => opening float is counted everywhere.
	expectedUSD, _, _, _, err := app.computeExpectedShiftTotals(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if expectedUSD != 15000 {
		t.Fatalf("default expected USD = %d, want 15000 (10000 sales + 5000 float)", expectedUSD)
	}
	printData, _, err := app.buildShiftClosePrintData(shift.ID, salesperson.ID, 0, 0, 0, 0, nil, closedAt, true)
	if err != nil {
		t.Fatal(err)
	}
	if printData.ShiftReport.OpeningFloatUSD != 5000 || printData.ShiftReport.ExpectedUSD != 15000 {
		t.Fatalf("default receipt: openingFloatUSD=%d expectedUSD=%d, want 5000 / 15000",
			printData.ShiftReport.OpeningFloatUSD, printData.ShiftReport.ExpectedUSD)
	}
	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := usdBalance(t, balances, "balance"); got != 15000 {
		t.Fatalf("default ListShiftBalances USD = %d, want 15000", got)
	}
	income, err := app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	if got := usdBalance(t, income, "total"); got != 15000 {
		t.Fatalf("default GetCurrentShiftIncomeByAccount USD = %d, want 15000", got)
	}

	// Flag off: opening float dropped from both the expectation and the receipt.
	if err := store.SetFeatureFlag(ctx, db.SetFeatureFlagParams{
		Key: countOpeningCashOnCloseFlag, Enabled: 0,
	}); err != nil {
		t.Fatal(err)
	}
	expectedUSD, _, _, _, err = app.computeExpectedShiftTotals(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if expectedUSD != 10000 {
		t.Fatalf("flag-off expected USD = %d, want 10000 (sales only)", expectedUSD)
	}
	printData, _, err = app.buildShiftClosePrintData(shift.ID, salesperson.ID, 0, 0, 0, 0, nil, closedAt, true)
	if err != nil {
		t.Fatal(err)
	}
	if printData.ShiftReport.OpeningFloatUSD != 0 || printData.ShiftReport.OpeningFloatLBP != 0 {
		t.Fatalf("flag-off receipt still shows opening float: USD=%d LBP=%d",
			printData.ShiftReport.OpeningFloatUSD, printData.ShiftReport.OpeningFloatLBP)
	}
	if printData.ShiftReport.ExpectedUSD != 10000 {
		t.Fatalf("flag-off receipt expectedUSD=%d, want 10000", printData.ShiftReport.ExpectedUSD)
	}
	balances, err = app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := usdBalance(t, balances, "balance"); got != 10000 {
		t.Fatalf("flag-off ListShiftBalances USD = %d, want 10000 (float excluded)", got)
	}
	income, err = app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	if got := usdBalance(t, income, "total"); got != 10000 {
		t.Fatalf("flag-off GetCurrentShiftIncomeByAccount USD = %d, want 10000 (float excluded)", got)
	}
}

// A shift that refunds more LBP than it takes in leaves expected LBP
// negative. computeExpectedShiftTotals must return that negative value (not
// error on the currency conversion), and closing the shift must still build
// its receipt. The counted-LBP suggestion is clamped to 0 on the frontend -
// physical cash can't go negative - so the variance simply shows the shortage.
func TestCloseShiftNegativeExpectedLBP(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}

	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "USD", CurrencyCode: "USD"}); err != nil {
		t.Fatal(err)
	}
	lbp, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "LBP", CurrencyCode: "LBP"})
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
	salesperson, err := store.CreateSalesperson(ctx, "Cashier")
	if err != nil {
		t.Fatal(err)
	}

	// A single LBP refund and no offsetting LBP sales this shift.
	refund, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: lbp.ID, ShiftID: shift.ID, InvoiceCode: "SA/RT/2026/NEGLBP",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: -30000, DiscountedTotal: -30000, Subtotal: -30000, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: refund.ID, CashboxID: lbp.ID, Amount: -30000,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: database}
	closedAt := time.Now().Format("2006-01-02 15:04:05")

	_, expectedLBP, _, _, err := app.computeExpectedShiftTotals(shift.ID)
	if err != nil {
		t.Fatalf("computeExpectedShiftTotals with negative LBP: %v", err)
	}
	if expectedLBP >= 0 {
		t.Fatalf("expected LBP = %d, want a negative value", expectedLBP)
	}

	printData, _, err := app.buildShiftClosePrintData(shift.ID, salesperson.ID, 0, 0, 0, 0, nil, closedAt, true)
	if err != nil {
		t.Fatalf("buildShiftClosePrintData with negative expected LBP: %v", err)
	}
	if printData.ShiftReport.ExpectedLBP != expectedLBP {
		t.Fatalf("receipt expectedLBP = %d, want %d", printData.ShiftReport.ExpectedLBP, expectedLBP)
	}
}

// The master opening_float_enabled switch turns the whole feature off: new
// shifts capture no float regardless of the configured default, the manual
// "set opening cash" endpoints refuse, and countOpeningCashOnClose is forced
// false so every total/receipt/balance already keyed off it ignores the float.
func TestOpeningFloatFeatureDisabled(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateDefaultOpeningFloat(ctx, db.UpdateDefaultOpeningFloatParams{
		DefaultOpeningFloatUsd: 7000,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "USD", CurrencyCode: "USD"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "LBP", CurrencyCode: "LBP"}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: database}

	// Feature on (default): a fresh shift picks up the configured default.
	on, err := app.OpenShift()
	if err != nil {
		t.Fatal(err)
	}
	if on["openingFloatUsd"].(int64) != 7000 {
		t.Fatalf("feature-on OpenShift float = %v, want 7000", on["openingFloatUsd"])
	}

	// Turn the whole feature off.
	if err := app.SetOpeningFloatEnabled(false); err != nil {
		t.Fatal(err)
	}
	if app.openingFloatEnabled() {
		t.Fatal("openingFloatEnabled() still true after SetOpeningFloatEnabled(false)")
	}
	if app.countOpeningCashOnClose() {
		t.Fatal("countOpeningCashOnClose() must be forced false while the feature is off")
	}

	// A new shift now starts with no float even though the default is 7000.
	off, err := app.OpenShift()
	if err != nil {
		t.Fatal(err)
	}
	if off["openingFloatUsd"].(int64) != 0 || off["openingFloatLbp"].(int64) != 0 {
		t.Fatalf("feature-off OpenShift float = %v/%v, want 0/0",
			off["openingFloatUsd"], off["openingFloatLbp"])
	}

	// The manual endpoints refuse while the feature is off.
	if _, err := app.ApplyOpeningFloatToCurrentShift(5000, 0, ""); err == nil {
		t.Fatal("ApplyOpeningFloatToCurrentShift should fail while the feature is off")
	}
	if err := app.UpdateDefaultOpeningFloat(5000, 0, ""); err == nil {
		t.Fatal("UpdateDefaultOpeningFloat should fail while the feature is off")
	}
}

// Expenses and loans paid out of the drawer during the shift come off what
// the cashier is expected to count, in the bucket of the account that paid
// them, and are listed on the printed report.
func TestCloseShiftExpectedSubtractsExpensesAndLoans(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}

	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	lbp, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "LBP", CurrencyCode: "LBP"})
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
	salesperson, err := store.CreateSalesperson(ctx, "Cashier")
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/SPEND",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 10000, DiscountedTotal: 10000, Subtotal: 10000, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 10000,
	}); err != nil {
		t.Fatal(err)
	}

	shiftRef := sql.NullInt64{Int64: shift.ID, Valid: true}
	if _, err := store.CreateExpense(ctx, db.CreateExpenseParams{
		ClientRef: "exp-1", Description: "internet", Amount: 2500,
		CurrencyCode: "USD", CashboxAccountID: usd.ID, ShiftID: shiftRef,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateLoan(ctx, db.CreateLoanParams{
		ClientRef: "loan-1", Description: "advance", Amount: 900_000 * 100,
		CurrencyCode: "LBP", CashboxAccountID: lbp.ID, ShiftID: shiftRef,
	}); err != nil {
		t.Fatal(err)
	}
	// Not this shift's: must not count.
	if _, err := store.CreateExpense(ctx, db.CreateExpenseParams{
		ClientRef: "exp-2", Description: "other shift", Amount: 999,
		CurrencyCode: "USD", CashboxAccountID: usd.ID,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: database}
	if err := store.SetFeatureFlag(ctx, db.SetFeatureFlagParams{
		Key: countOpeningCashOnCloseFlag, Enabled: 0,
	}); err != nil {
		t.Fatal(err)
	}

	expectedUSD, expectedLBP, _, _, err := app.computeExpectedShiftTotals(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if expectedUSD != 7500 {
		t.Fatalf("expected USD = %d, want 7500 (10000 sales - 2500 expense)", expectedUSD)
	}
	if expectedLBP != -900_000 {
		t.Fatalf("expected LBP = %d, want -900000 (LBP loan)", expectedLBP)
	}

	closedAt := time.Now().Format("2006-01-02 15:04:05")
	printData, _, err := app.buildShiftClosePrintData(shift.ID, salesperson.ID, 0, 0, 0, 0, nil, closedAt, true)
	if err != nil {
		t.Fatal(err)
	}
	report := printData.ShiftReport
	if len(report.Spend) != 2 || report.PaidOutUSD != 2500 || report.PaidOutLBP != 900_000 {
		t.Fatalf("receipt spend = %+v paidOutUSD=%d paidOutLBP=%d", report.Spend, report.PaidOutUSD, report.PaidOutLBP)
	}
	if report.ExpectedUSD != 7500 {
		t.Fatalf("receipt expected USD = %d, want 7500", report.ExpectedUSD)
	}
}
