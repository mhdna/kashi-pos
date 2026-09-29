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
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
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
	// 41498, not rounded to 41500: the summed payment amount is already an
	// exact, checkout-validated cents value (matching the invoice's actual
	// recorded total) - re-rounding the sum was redundant and could make
	// this page disagree with the invoice it's summing.
	if todayByAccount["USD"] != 41498 || todayByAccount["LBP"] != 2000 {
		t.Fatalf("today account totals = %#v", todayByAccount)
	}
	currentShiftIncome, err := app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	var currentShiftIncomeTotal int64
	for _, row := range currentShiftIncome {
		currentShiftIncomeTotal += databaseInt64(row["total"])
	}
	if currentShiftIncomeTotal != 43498 {
		t.Fatalf("current shift income total = %d, want 43498", currentShiftIncomeTotal)
	}

	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	shiftByAccount := incomeRowsByAccount(balances, "balance")
	// See the comment above on GetTodayIncomeByAccount: 41498, not
	// re-rounded to 41500.
	if shiftByAccount["USD"] != 41498 || shiftByAccount["LBP"] != 2000 {
		t.Fatalf("shift account totals = %#v", shiftByAccount)
	}

	days, err := app.GetDailyIncomeByMonth(time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0]["total"] != float64(43498) {
		t.Fatalf("monthly totals = %#v", days)
	}

	shifts, err := app.GetShiftTotals("")
	if err != nil {
		t.Fatal(err)
	}
	if len(shifts) != 1 || shifts[0]["total"] != float64(43498) {
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
	if report.ExpectedUSD != 41498 || report.ExpectedLBP != 1800000 || report.TotalPayments != 43498 || report.NetSales != 43498 {
		t.Fatalf("receipt report totals = %#v", report)
	}
	receiptByAccount := incomeRowsByAccount(receiptPaymentsToRows(report.Payments), "total")
	if receiptByAccount["USD"] != 41498 || receiptByAccount["LBP"] != 2000 {
		t.Fatalf("receipt account totals = %#v", receiptByAccount)
	}
}

// A "calculate on checkout" coupon type is settled later by whoever issued
// it (e.g. a supplier reimbursement), not collected by the branch when the
// coupon is redeemed - so its payment amount must not inflate the shift's
// net sales even though it's part of the invoice's grand_total.
func TestNetSalesExcludesCalculateOnCheckoutCouponPayments(t *testing.T) {
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

	deferredType, err := store.CreateCouponType(ctx, db.CreateCouponTypeParams{
		Name: "Supplier Reimbursed", ValueCents: 2000, CurrencyCode: "USD", IsActive: 1, CalculateOnCheckout: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00099",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 5000, DiscountedTotal: 5000, Subtotal: 5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInvoiceCouponPayment(ctx, db.CreateInvoiceCouponPaymentParams{
		InvoiceID: invoice.ID, CouponCode: "SUP-20", CouponTypeID: deferredType.ID, Amount: 2000,
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := store.GetShiftReportSummary(ctx, shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.NetSales != 3000 {
		t.Fatalf("net sales = %d, want 5000 grand total minus 2000 deferred coupon = 3000", summary.NetSales)
	}
}

// The balance page's "total income" figures (monthly income chart and
// branch-target progress) must exclude deferred (calculate-on-checkout)
// coupon payments too, the same way the shift report's net sales does -
// otherwise a branch could look like it hit its revenue target purely off
// supplier-reimbursed vouchers it hasn't actually been paid for yet.
func TestBalancePageIncomeExcludesCalculateOnCheckoutCouponPayments(t *testing.T) {
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

	deferredType, err := store.CreateCouponType(ctx, db.CreateCouponTypeParams{
		Name: "Supplier Reimbursed", ValueCents: 2000, CurrencyCode: "USD", IsActive: 1, CalculateOnCheckout: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00100",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 5000, DiscountedTotal: 5000, Subtotal: 5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInvoiceCouponPayment(ctx, db.CreateInvoiceCouponPaymentParams{
		InvoiceID: invoice.ID, CouponCode: "SUP-20", CouponTypeID: deferredType.ID, Amount: 2000,
	}); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	total, err := store.GetInvoiceIncomeTotalByDate(ctx, db.GetInvoiceIncomeTotalByDateParams{
		CreatedAt: today, CreatedAt_2: today,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3000 {
		t.Fatalf("GetInvoiceIncomeTotalByDate = %d, want 5000 grand total minus 2000 deferred coupon = 3000", total)
	}

	rangeTotal, err := store.GetInvoiceIncomeTotalByDateRange(ctx, db.GetInvoiceIncomeTotalByDateRangeParams{
		DateFrom: today, DateTo: today,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rangeTotal != 3000 {
		t.Fatalf("GetInvoiceIncomeTotalByDateRange = %d, want 3000", rangeTotal)
	}

	monthRows, err := store.ListDailyInvoiceIncomeByMonth(ctx, time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(monthRows) != 1 || monthRows[0].Total != 3000 {
		t.Fatalf("ListDailyInvoiceIncomeByMonth = %#v, want a single day totalling 3000", monthRows)
	}

	// The shift list's "total" column (shown on the same balance page as the
	// income chart above) must agree with it, or the page shows two
	// different numbers for the same day's income.
	shiftRows, err := store.GetShiftTotals(ctx, time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(shiftRows) != 1 || shiftRows[0].Total != 3000 {
		t.Fatalf("GetShiftTotals = %#v, want a single shift totalling 3000", shiftRows)
	}

	// The balance page surfaces the excluded amount separately (a caption
	// under the total) instead of just silently showing a lower number.
	app := &App{ctx: ctx, store: store}
	excluded, err := app.GetDeferredCouponTotalByMonth(time.Now().Format("2006-01"))
	if err != nil {
		t.Fatal(err)
	}
	if excluded != 2000 {
		t.Fatalf("GetDeferredCouponTotalByMonth = %v, want 2000", excluded)
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
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
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
	shifts, err := app.GetShiftTotals("")
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
