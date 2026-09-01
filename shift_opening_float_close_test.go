package main

import (
	"context"
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
