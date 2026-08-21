package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
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

func TestSettleInvoicePaymentsPreservesGrandTotalAcrossCurrencies(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settle.db")+"?_pragma=foreign_keys(ON)")
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
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00004",
		InvoiceIndex: 4, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSalesInvoice(ctx, invoice.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePaymentEntry(ctx, db.CreateInvoicePaymentEntryParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1000, EntryType: "payment",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
		AccountID: usd.ID, ShiftID: shift.ID, Balance: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	newPayments, _ := json.Marshal([]checkoutPayment{
		{AccountID: lbp.ID, AmountCents: 1000, EntryType: "payment"},
	})
	if _, err := app.SettleInvoicePayments(invoice.ID, string(newPayments)); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetInvoiceByID(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GrandTotal != 1000 {
		t.Fatalf("grand total changed to %d, want 1000", updated.GrandTotal)
	}
	if updated.CashboxID != lbp.ID {
		t.Fatalf("invoice cashbox = %d, want %d", updated.CashboxID, lbp.ID)
	}
	payments, err := store.ListInvoicePayments(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].CashboxID != lbp.ID || payments[0].Amount != 1000 {
		t.Fatalf("updated payments = %#v, want a single LBP payment of 1000 cents", payments)
	}
	usdBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{AccountID: usd.ID, ShiftID: shift.ID})
	if err != nil {
		t.Fatal(err)
	}
	lbpBalance, err := store.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{AccountID: lbp.ID, ShiftID: shift.ID})
	if err != nil {
		t.Fatal(err)
	}
	if usdBalance.Balance != 0 || lbpBalance.Balance != 1000 {
		t.Fatalf("balances after settlement change = USD %d, LBP %d", usdBalance.Balance, lbpBalance.Balance)
	}
}

func TestSettleInvoicePaymentsRejectsMismatchedTotal(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settle-mismatch.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	shift, _ := store.CreateShift(ctx, 0)
	usd, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00005",
		InvoiceIndex: 5, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSalesInvoice(ctx, invoice.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	underpaid, _ := json.Marshal([]checkoutPayment{{AccountID: usd.ID, AmountCents: 500, EntryType: "payment"}})
	if _, err := app.SettleInvoicePayments(invoice.ID, string(underpaid)); err == nil {
		t.Fatal("expected an under-paid settlement to be rejected")
	}

	// The rejected attempt must not have partially applied - the original
	// payment should still be intact.
	payments, err := store.ListInvoicePayments(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 || payments[0].Amount != 1000 {
		t.Fatalf("payments after rejected settlement = %#v, want the original 1000-cent payment untouched", payments)
	}
}

func TestSettleInvoicePaymentsRejectsNonSalesInvoice(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settle-nonsales.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	shift, _ := store.CreateShift(ctx, 0)
	usd, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	// Intentionally not marked via CreateSalesInvoice - e.g. an
	// exchange/return invoice, which must never be reopened for settlement
	// change.
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "EX/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 500, DiscountedTotal: 500, GrandTotal: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: usd.ID, AmountCents: 500, EntryType: "payment"}})
	if _, err := app.SettleInvoicePayments(invoice.ID, string(payments)); err == nil || !strings.Contains(err.Error(), "sales invoice not found") {
		t.Fatalf("SettleInvoicePayments on a non-sales invoice = %v, want a sales-invoice-not-found error", err)
	}
	if _, err := app.GetInvoiceForSettlementChange(invoice.ID); err == nil || !strings.Contains(err.Error(), "sales invoice not found") {
		t.Fatalf("GetInvoiceForSettlementChange on a non-sales invoice = %v, want a sales-invoice-not-found error", err)
	}
}

func TestSettleInvoicePaymentsSupportsOnAccount(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settle-onaccount.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	shift, _ := store.CreateShift(ctx, 0)
	usd, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	onAccount, _ := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	inventory, _ := store.CreateInventory(ctx, "Sales")
	client, _ := store.CreateClient(ctx, db.CreateClientParams{Name: "Jane Regular", Phone: "70000000"})
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00006",
		InvoiceIndex: 6, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 1000, DiscountedTotal: 1000, GrandTotal: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSalesInvoice(ctx, invoice.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: usd.ID, Amount: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
		AccountID: usd.ID, ShiftID: shift.ID, Balance: 1000,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	newPayments, _ := json.Marshal([]checkoutPayment{{AccountID: onAccount.ID, AmountCents: 1000, EntryType: "payment"}})
	if _, err := app.SettleInvoicePayments(invoice.ID, string(newPayments)); err != nil {
		t.Fatal(err)
	}
	entry, err := store.GetClientAccountEntryByInvoiceID(ctx, invoice.ID)
	if err != nil {
		t.Fatalf("expected an on-account client entry to be created: %v", err)
	}
	if entry.ClientID != client.ID || entry.Amount != 1000 {
		t.Fatalf("client account entry = %#v, want client %d amount 1000", entry, client.ID)
	}
}
