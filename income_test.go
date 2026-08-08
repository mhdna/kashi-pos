package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestIncomeViewsUseLiveInvoicePaymentsAfterDeletion(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "income.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	shift, err := store.CreateShift(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{
		Name: "Cash", CurrencyCode: "USD",
	})
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
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 1000, DiscountedTotal: 1000, Subtotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: cashbox.ID, Amount: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
		AccountID: cashbox.ID, ShiftID: shift.ID, Balance: 9999,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	assertIncomeTotals(t, app, shift.ID, 1000)

	monthRows, err := app.GetDailyIncomeByMonth(time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(monthRows) != 1 || monthRows[0]["total"] != float64(1000) {
		t.Fatalf("monthly income = %#v", monthRows)
	}

	if err := app.DeleteInvoice(invoice.ID); err != nil {
		t.Fatal(err)
	}
	assertIncomeTotals(t, app, shift.ID, 0)
}

func TestIncomeViewsUseIdenticalMixedCurrencyAccountTotals(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, 0)
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
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/MIXED",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 43498, DiscountedTotal: 43498, Subtotal: 43498, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, payment := range []db.CreateInvoicePaymentParams{
		{InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 41498},
		{InvoiceID: invoice.ID, CashboxID: lbp.ID, Amount: 2000},
	} {
		if err := store.CreateInvoicePayment(ctx, payment); err != nil {
			t.Fatal(err)
		}
	}

	app := &App{ctx: ctx, store: store, conn: database}
	today, err := app.GetTodayIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	todayByAccount := incomeRowsByAccount(today, "total")
	if todayByAccount["USD"] != 41500 || todayByAccount["LBP"] != 2000 {
		t.Fatalf("today account totals = %#v", todayByAccount)
	}

	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	shiftByAccount := incomeRowsByAccount(balances, "balance")
	if shiftByAccount["USD"] != 41500 || shiftByAccount["LBP"] != 2000 {
		t.Fatalf("shift account totals = %#v", shiftByAccount)
	}

	days, err := app.GetDailyIncomeByMonth(time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0]["total"] != float64(43500) {
		t.Fatalf("monthly totals = %#v", days)
	}

	shifts, err := app.GetShiftTotals()
	if err != nil {
		t.Fatal(err)
	}
	if len(shifts) != 1 || shifts[0]["total"] != float64(43500) {
		t.Fatalf("shift grand total = %#v", shifts)
	}

	printData, _, err := app.buildShiftClosePrintData(
		shift.ID, salesperson.ID, 0, 0, 0, 0, nil,
		time.Now().Format("2006-01-02 15:04:05"), true,
	)
	if err != nil {
		t.Fatal(err)
	}
	report := printData.ShiftReport
	if report.ExpectedUSD != 41498 || report.ExpectedLBP != 1800000 || report.TotalPayments != 43498 {
		t.Fatalf("receipt report totals = %#v", report)
	}
	receiptByAccount := incomeRowsByAccount(receiptPaymentsToRows(report.Payments), "total")
	if receiptByAccount["USD"] != 41498 || receiptByAccount["LBP"] != 2000 {
		t.Fatalf("receipt account totals = %#v", receiptByAccount)
	}
}

func incomeRowsByAccount(rows []map[string]any, amountKey string) map[string]int64 {
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		switch value := row[amountKey].(type) {
		case float64:
			result[row["accountName"].(string)] = int64(value)
		default:
			result[row["accountName"].(string)] = databaseInt64(value)
		}
	}
	return result
}

func receiptPaymentsToRows(payments []receiptPayment) []map[string]any {
	rows := make([]map[string]any, len(payments))
	for i, payment := range payments {
		rows[i] = map[string]any{"accountName": payment.AccountName, "total": payment.AmountCents}
	}
	return rows
}

func TestIncomeTraceShowsSignedWhishChange(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "income-trace.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	shift, err := store.CreateShift(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	whish, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Whish", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Trace Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Trace Client", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: whish.ID, ShiftID: shift.ID, InvoiceCode: "SA/2026/TRACE", InvoiceIndex: 1,
		Year: 2026, ClientID: client.ID, InventoryID: inventory.ID, GrandTotal: 1000, DiscountedTotal: 1000, Subtotal: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var invoiceID int64
	if err := database.QueryRowContext(ctx, "SELECT id FROM invoices WHERE invoice_code = ?", "SA/2026/TRACE").Scan(&invoiceID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoiceID, CashboxID: whish.ID, Amount: -4}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePaymentEntry(ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoiceID, CashboxID: whish.ID, Amount: -4, EntryType: "change"}); err != nil {
		t.Fatal(err)
	}

	trace, err := (&App{ctx: ctx, store: store, conn: database}).incomeTrace(ctx, time.Now().Format("2006-01-02"), "Whish")
	if err != nil {
		t.Fatal(err)
	}
	if trace.TotalCents != -4 || len(trace.Contributions) != 1 {
		t.Fatalf("trace = %#v, want one -4-cent contribution", trace)
	}
	item := trace.Contributions[0]
	if item.EntryType != "change" || item.InvoiceCode != "SA/2026/TRACE" || item.AmountCents != -4 {
		t.Fatalf("trace contribution = %#v", item)
	}
}

func assertIncomeTotals(t *testing.T, app *App, shiftID int64, want float64) {
	t.Helper()

	today, err := app.GetTodayIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	balances, err := app.ListShiftBalances(shiftID)
	if err != nil {
		t.Fatal(err)
	}
	shifts, err := app.GetShiftTotals()
	if err != nil {
		t.Fatal(err)
	}

	var todayTotal, balanceTotal, shiftTotal float64
	for _, row := range today {
		todayTotal += row["total"].(float64)
	}
	for _, row := range balances {
		balanceTotal += float64(row["balance"].(int64))
	}
	for _, row := range shifts {
		if row["id"] == shiftID {
			shiftTotal = row["total"].(float64)
		}
	}
	if todayTotal != want || balanceTotal != want || shiftTotal != want {
		t.Fatalf(
			"income totals: today=%.0f balances=%.0f shift=%.0f, want %.0f",
			todayTotal, balanceTotal, shiftTotal, want,
		)
	}
}
