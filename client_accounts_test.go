package main

import (
	"context"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func TestClientAccountPaymentBecomesCollectedIncomeWhenReceived(t *testing.T) {
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
	onAccount, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	cash, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Customer", Phone: "+96171111111"})
	salesperson, _ := store.CreateSalesperson(ctx, "Cashier")
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: onAccount.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: onAccount.ID, Amount: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateClientAccountEntry(ctx, db.CreateClientAccountEntryParams{ClientID: client.ID, InvoiceID: invoice.ID, Amount: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE invoices SET created_at = datetime('now', '-4 days') WHERE id = ?`, invoice.ID); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	before, err := app.GetShiftTotals()
	if err != nil || len(before) != 1 || databaseInt64(before[0]["total"]) != 0 {
		t.Fatalf("collected total before payment = %#v, err=%v", before, err)
	}
	if _, err := app.PayClientAccount(invoice.ID, cash.ID, 400); err != nil {
		t.Fatal(err)
	}
	details, err := app.GetClientAccountDetails(client.ID)
	if err != nil || databaseInt64(details["balance"]) != 600 {
		t.Fatalf("account details after payment = %#v, err=%v", details, err)
	}
	after, err := app.GetShiftTotals()
	if err != nil || after[0]["total"].(float64) != 400 {
		t.Fatalf("collected total after payment = %#v, err=%v", after, err)
	}
	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, balance := range balances {
		got[balance["accountName"].(string)] = databaseInt64(balance["balance"])
	}
	if got["On Account"] != 1000 || got["Cash USD"] != 400 {
		t.Fatalf("shift balances = %#v", balances)
	}
	today, err := app.GetTodayIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	todayTotals := map[string]float64{}
	for _, row := range today {
		todayTotals[row["accountName"].(string)] = row["total"].(float64)
	}
	if todayTotals["On Account"] != 600 || todayTotals["Cash USD"] != 400 {
		t.Fatalf("today income rows = %#v", today)
	}
	days, err := app.GetDailyIncomeByMonth(time.Now().Format("2006-01"))
	if err != nil || len(days) != 1 || days[0]["total"].(float64) != 400 {
		t.Fatalf("monthly collected income = %#v, err=%v", days, err)
	}
	printData, _, err := app.buildShiftClosePrintData(
		shift.ID, salesperson.ID, 0, 0, 0, 0, nil, time.Now().Format("2006-01-02 15:04:05"), true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if printData.ShiftReport.TotalPayments != 400 || printData.ShiftReport.OnAccount != 1000 {
		t.Fatalf("shift report totals = %#v", printData.ShiftReport)
	}
	if _, err := app.PayClientAccount(invoice.ID, cash.ID, 601); err == nil {
		t.Fatal("payment above the outstanding balance was accepted")
	}
}

func TestTodayIncomeIncludesOnAccountPartOfSplitPayment(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	shift, err := store.CreateShift(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	onAccount, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	cash, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Split Customer", Phone: "+96172222222"})
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: cash.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00002",
		InvoiceIndex: 2, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, payment := range []db.CreateInvoicePaymentParams{
		{InvoiceID: invoice.ID, CashboxID: cash.ID, Amount: 400},
		{InvoiceID: invoice.ID, CashboxID: onAccount.ID, Amount: 600},
	} {
		if err := store.CreateInvoicePayment(ctx, payment); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.CreateClientAccountEntry(ctx, db.CreateClientAccountEntryParams{
		ClientID: client.ID, InvoiceID: invoice.ID, Amount: 600,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	today, err := app.GetTodayIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]float64{}
	for _, row := range today {
		totals[row["accountName"].(string)] = row["total"].(float64)
	}
	if totals["Cash USD"] != 400 || totals["On Account"] != 600 {
		t.Fatalf("today income rows = %#v", today)
	}
}
