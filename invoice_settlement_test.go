package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestChangeInvoiceSettlementPreservesPaidValueAcrossCurrencies(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settlement.db")+"?_pragma=foreign_keys(ON)")
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
	lbp, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash LBP", CurrencyCode: "LBP"})
	if err != nil {
		t.Fatal(err)
	}
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
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
		CashboxID: lbp.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Payment amounts use the accounting currency (USD cents). At 90,000 LBP
	// per USD, the physical 900,000 LBP payment is represented as 1,000 cents.
	const paidCents int64 = 1000
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: lbp.ID, Amount: paidCents,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePaymentEntry(ctx, db.CreateInvoicePaymentEntryParams{
		InvoiceID: invoice.ID, CashboxID: lbp.ID, Amount: paidCents, EntryType: "payment",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
		AccountID: lbp.ID, ShiftID: shift.ID, Balance: paidCents,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	if err := app.ChangeInvoiceSettlement(invoice.ID, usd.ID); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetInvoiceByID(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CashboxID != usd.ID {
		t.Fatalf("invoice cashbox = %d, want %d", updated.CashboxID, usd.ID)
	}
	payments, err := store.ListInvoicePayments(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].CashboxID != usd.ID || payments[0].Amount != paidCents {
		t.Fatalf("updated payments = %#v, want USD payment of %d cents", payments, paidCents)
	}
	oldBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{
		AccountID: lbp.ID, ShiftID: shift.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	newBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{
		AccountID: usd.ID, ShiftID: shift.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if oldBalance.Balance != 0 || newBalance.Balance != paidCents {
		t.Fatalf("balances after settlement change = LBP %d, USD %d", oldBalance.Balance, newBalance.Balance)
	}
}

func TestChangeInvoiceSettlementPreservesOverpayment(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "overpayment.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	shift, _ := store.CreateShift(ctx, 0)
	oldAccount, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Old", CurrencyCode: "LBP"})
	newAccount, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "New", CurrencyCode: "USD"})
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: oldAccount.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00002",
		InvoiceIndex: 2, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: oldAccount.ID, Amount: 1200,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	if err := app.ChangeInvoiceSettlement(invoice.ID, newAccount.ID); err != nil {
		t.Fatal(err)
	}
	payments, err := store.ListInvoicePayments(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].Amount != 1200 {
		t.Fatalf("updated payments = %#v, want preserved 1200-cent payment", payments)
	}
}

func TestChangeInvoiceSettlementAmountMovesOnlyRequestedValue(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "partial-settlement.db")+"?_pragma=foreign_keys(ON)")
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
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	lbp, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash LBP", CurrencyCode: "LBP"})
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
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00003",
		InvoiceIndex: 3, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1500, DiscountedTotal: 1500, GrandTotal: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1500,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePaymentEntry(ctx, db.CreateInvoicePaymentEntryParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1500, EntryType: "payment",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
		AccountID: usd.ID, ShiftID: shift.ID, Balance: 1500,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	if err := app.ChangeInvoiceSettlementAmount(invoice.ID, usd.ID, lbp.ID, 1000); err != nil {
		t.Fatal(err)
	}

	payments, err := store.ListInvoicePayments(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[int64]int64, len(payments))
	for _, payment := range payments {
		got[payment.CashboxID] = payment.Amount
	}
	if got[usd.ID] != 500 || got[lbp.ID] != 1000 {
		t.Fatalf("partial settlement = %#v, want USD=500 and LBP=1000", got)
	}
	usdBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{
		AccountID: usd.ID, ShiftID: shift.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	lbpBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{
		AccountID: lbp.ID, ShiftID: shift.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if usdBalance.Balance != 500 || lbpBalance.Balance != 1000 {
		t.Fatalf(
			"partial balances = USD %d, LBP %d, want USD 500 and LBP 1000",
			usdBalance.Balance, lbpBalance.Balance,
		)
	}
	updatedInvoice, err := store.GetInvoiceByID(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedInvoice.CashboxID != usd.ID {
		t.Fatalf("primary cashbox changed during partial move: got %d, want %d", updatedInvoice.CashboxID, usd.ID)
	}
	page, err := app.ListInvoicesPage(1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("invoice page contains %d items, want 1", len(page.Items))
	}
	accounts, ok := page.Items[0]["settlementAccounts"].([]string)
	if !ok || len(accounts) != 2 || accounts[0] != "Cash USD" || accounts[1] != "Cash LBP" {
		t.Fatalf("invoice settlement accounts = %#v, want Cash USD and Cash LBP", page.Items[0]["settlementAccounts"])
	}

	if err := app.ChangeInvoiceSettlementAmount(invoice.ID, usd.ID, lbp.ID, 501); err == nil {
		t.Fatal("expected moving more than the remaining source settlement to fail")
	}
}
