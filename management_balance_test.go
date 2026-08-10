package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

type balanceSwapFixture struct {
	app           *App
	database      *sql.DB
	shiftID       int64
	sourceID      int64
	destinationID int64
	onAccountID   int64
	onCouponID    int64
}

func newBalanceSwapFixture(t *testing.T, total int64) balanceSwapFixture {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "swap.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
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
	account := func(name string) db.CashboxAccount {
		t.Helper()
		value, createErr := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: name, CurrencyCode: "USD"})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return value
	}
	source := account("Cash")
	destination := account("Card")
	onAccount := account("On Account")
	onCoupon := account("On Coupon")
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: source.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: total, DiscountedTotal: total, Subtotal: total,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: source.ID, Amount: total}); err != nil {
		t.Fatal(err)
	}
	return balanceSwapFixture{
		app: &App{ctx: ctx, store: store}, database: database, shiftID: shift.ID,
		sourceID: source.ID, destinationID: destination.ID, onAccountID: onAccount.ID, onCouponID: onCoupon.ID,
	}
}

func fixtureBalances(t *testing.T, fixture balanceSwapFixture) map[int64]int64 {
	t.Helper()
	rows, err := fixture.app.ListShiftBalances(fixture.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	result := map[int64]int64{}
	for _, row := range rows {
		result[row["accountId"].(int64)] = databaseInt64(row["balance"])
	}
	return result
}

func replacementCount(t *testing.T, database *sql.DB) int64 {
	t.Helper()
	var count int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM shift_balance_replacements`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestReplaceShiftBalancePreservesTotalAcrossPartialAndReverseSwaps(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 400); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 600 || balances[fixture.destinationID] != 400 {
		t.Fatalf("partial swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.destinationID, fixture.sourceID, 150); err != nil {
		t.Fatal(err)
	}
	balances = fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 750 || balances[fixture.destinationID] != 250 {
		t.Fatalf("reverse swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if balances[fixture.sourceID]+balances[fixture.destinationID] != 1000 {
		t.Fatalf("swap changed total balance: %#v", balances)
	}
	var originalPayment int64
	if err := fixture.database.QueryRow(`SELECT amount FROM invoice_payments LIMIT 1`).Scan(&originalPayment); err != nil {
		t.Fatal(err)
	}
	if originalPayment != 1000 {
		t.Fatalf("swap changed original payment to %d", originalPayment)
	}
}

func TestReplaceShiftBalancePreservesNegativeTotalAndRejectsOverdraw(t *testing.T) {
	fixture := newBalanceSwapFixture(t, -1000)
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 400); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != -600 || balances[fixture.destinationID] != -400 {
		t.Fatalf("negative partial swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 601); err == nil {
		t.Fatal("negative source overdraw was accepted")
	}
	if replacementCount(t, fixture.database) != 1 {
		t.Fatal("rejected overdraw wrote a replacement record")
	}
}

func TestReplaceShiftBalanceRejectsInvalidAndNonCashAccountsWithoutWriting(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)
	cases := []struct {
		name     string
		from, to int64
		amount   int64
	}{
		{name: "zero amount", from: fixture.sourceID, to: fixture.destinationID, amount: 0},
		{name: "negative amount", from: fixture.sourceID, to: fixture.destinationID, amount: -1},
		{name: "same account", from: fixture.sourceID, to: fixture.sourceID, amount: 1},
		{name: "above balance", from: fixture.sourceID, to: fixture.destinationID, amount: 1001},
		{name: "on account destination", from: fixture.sourceID, to: fixture.onAccountID, amount: 1},
		{name: "coupon destination", from: fixture.sourceID, to: fixture.onCouponID, amount: 1},
		{name: "missing destination", from: fixture.sourceID, to: 999999, amount: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, test.from, test.to, test.amount); err == nil {
				t.Fatal("invalid replacement was accepted")
			}
		})
	}
	if replacementCount(t, fixture.database) != 0 {
		t.Fatal("invalid replacements wrote records")
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 1000 || balances[fixture.destinationID] != 0 {
		t.Fatalf("invalid replacements changed balances: %#v", balances)
	}
}

func TestReplaceShiftBalanceMovesNegativeBalanceWithItsSign(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "balance.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
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
	source, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	destination, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Card", CurrencyCode: "USD"})
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
		CashboxID: source.ID, ShiftID: shift.ID, InvoiceCode: "RS/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: -1000, DiscountedTotal: -1000, Subtotal: -1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: source.ID, Amount: -1000,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	if err := app.ReplaceShiftBalance(shift.ID, source.ID, destination.ID, 1000); err != nil {
		t.Fatal(err)
	}
	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]int64{}
	for _, balance := range balances {
		got[balance["accountId"].(int64)] = databaseInt64(balance["balance"])
	}
	if got[source.ID] != 0 || got[destination.ID] != -1000 {
		t.Fatalf("balances after negative replacement = source %d, destination %d", got[source.ID], got[destination.ID])
	}
	currentShift, err := app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	todayBalances := map[int64]int64{}
	for _, balance := range currentShift {
		todayBalances[balance["accountId"].(int64)] = int64(balance["total"].(float64))
	}
	if todayBalances[source.ID] != 0 || todayBalances[destination.ID] != -1000 {
		t.Fatalf("current shift balances after negative replacement = source %d, destination %d", todayBalances[source.ID], todayBalances[destination.ID])
	}
}
