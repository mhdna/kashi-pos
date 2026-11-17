package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// onAccountReturnFixture: the remote-return till plus an On Account method and a
// named client, with a sale of 2 x $10 put on account.
type onAccountReturnFixture struct {
	remoteReturnFixture
	onAccount   db.CashboxAccount
	client      db.Client
	salesperson db.Salesperson
	saleID      int64
}

func newOnAccountReturnFixture(t *testing.T) onAccountReturnFixture {
	t.Helper()
	ctx := context.Background()
	f := newRemoteReturnFixture(t)
	onAccount, err := f.store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	client, err := f.store.CreateClient(ctx, db.CreateClientParams{Name: "Maya", Phone: "+96170123456"})
	if err != nil {
		t.Fatal(err)
	}
	salesperson, err := f.store.CreateSalesperson(ctx, "On Account Cashier")
	if err != nil {
		t.Fatal(err)
	}
	items, _ := json.Marshal([]salesInvoiceItem{{Code: f.product.Code, Barcode: f.product.Barcode, Qty: 2, SalespersonID: salesperson.ID}})
	pays, _ := json.Marshal([]checkoutPayment{{AccountID: onAccount.ID, AmountCents: 2000}})
	sale, err := f.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(pays), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := f.app.GetExchangeInvoice(sale.InvoiceCode)
	if err != nil {
		t.Fatal(err)
	}
	return onAccountReturnFixture{remoteReturnFixture: f, onAccount: onAccount, client: client, salesperson: salesperson, saleID: loaded["id"].(int64)}
}

var onAccountReturnPolicy = exchangePolicy{allowLate: true, allowLowerTotal: true, returnOnly: true}

// returnUnits returns qty units of the sale, refunded as the cashier entered.
func (f onAccountReturnFixture) returnUnits(qty int64, payments []checkoutPayment, settlement int64) error {
	items := fmt.Sprintf(`[{"code":%q,"barcode":%q,"qty":%d}]`, f.product.Code, f.product.Barcode, -qty)
	pays := ""
	if payments != nil {
		raw, _ := json.Marshal(payments)
		pays = string(raw)
	}
	_, err := f.app.checkoutReturnExchangeInvoice(items, pays, f.saleID, f.salesperson.ID, "", settlement, onAccountReturnPolicy)
	return err
}

func (f onAccountReturnFixture) owed(t *testing.T) int64 {
	t.Helper()
	details, err := f.app.GetClientAccountDetails(f.client.ID)
	if err != nil {
		t.Fatal(err)
	}
	return databaseInt64(details["balance"])
}

func (f onAccountReturnFixture) listedOwed(t *testing.T) int64 {
	t.Helper()
	list, err := f.app.ListOnAccountClients("")
	if err != nil {
		t.Fatal(err)
	}
	return databaseInt64(list["outstanding"])
}

func (f onAccountReturnFixture) shiftBalance(t *testing.T, accountID int64) int64 {
	t.Helper()
	var balance int64
	err := f.database.QueryRow(`SELECT COALESCE(SUM(balance), 0) FROM shifts_accounts_balances WHERE account_id = ?`, accountID).Scan(&balance)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

// The reported case: an unpaid on-account sale returned with a cash refund
// kept the whole debt and took cash out of the drawer.
func TestReturnOfUnpaidOnAccountSaleComesOffTheDebt(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	if got := f.owed(t); got != 2000 {
		t.Fatalf("owed before = %d", got)
	}
	cash := f.shiftBalance(t, f.cashbox.ID)
	if err := f.returnUnits(2, []checkoutPayment{{AccountID: f.cashbox.ID, AmountCents: -2000, EntryType: "change"}}, 0); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed after full return = %d, want 0", got)
	}
	if got := f.listedOwed(t); got != 0 {
		t.Fatalf("On Account page total = %d, want 0", got)
	}
	if got := f.shiftBalance(t, f.cashbox.ID) - cash; got != 0 {
		t.Fatalf("cash moved by %d, want 0: no money was given back", got)
	}
	if got := f.shiftBalance(t, f.onAccount.ID); got != 0 {
		t.Fatalf("On Account shift balance = %d, want 0 (2000 sold, 2000 returned)", got)
	}
}

func TestPartReturnOfOnAccountSaleLeavesTheRestOwed(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	if err := f.returnUnits(1, []checkoutPayment{{AccountID: f.cashbox.ID, AmountCents: -1000, EntryType: "change"}}, 0); err != nil {
		t.Fatal(err)
	}
	if got, list := f.owed(t), f.listedOwed(t); got != 1000 || list != 1000 {
		t.Fatalf("owed = %d, page = %d, want 1000 each", got, list)
	}
	// Paying the rest clears it; a payment over it is refused.
	if _, err := f.app.PayClientAccount(f.saleID, f.cashbox.ID, 1001); err == nil {
		t.Fatal("paying more than is left owed should fail")
	}
	if _, err := f.app.PayClientAccount(f.saleID, f.cashbox.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed after paying the rest = %d", got)
	}
}

// Paid $15 of $20 back, then returned it all: $5 comes off the debt and only
// the $15 they paid goes back in cash.
func TestReturnOfPartlyPaidOnAccountSaleRefundsOnlyWhatWasPaid(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	cash := f.shiftBalance(t, f.cashbox.ID)
	if _, err := f.app.PayClientAccount(f.saleID, f.cashbox.ID, 1500); err != nil {
		t.Fatal(err)
	}
	if err := f.returnUnits(2, []checkoutPayment{{AccountID: f.cashbox.ID, AmountCents: -2000, EntryType: "change"}}, 0); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed = %d, want 0", got)
	}
	if got := f.shiftBalance(t, f.cashbox.ID) - cash; got != 0 {
		t.Fatalf("cash moved by %d, want 0 (+1500 paid back, -1500 refunded)", got)
	}
	// Two refund methods can't be split automatically.
	g := newOnAccountReturnFixture(t)
	if _, err := g.app.PayClientAccount(g.saleID, g.cashbox.ID, 1500); err != nil {
		t.Fatal(err)
	}
	other, _ := g.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{Name: "Card", CurrencyCode: "USD"})
	err := g.returnUnits(2, []checkoutPayment{
		{AccountID: g.cashbox.ID, AmountCents: -1000, EntryType: "change"},
		{AccountID: other.ID, AmountCents: -1000, EntryType: "change"},
	}, 0)
	if err == nil || !strings.Contains(err.Error(), "one payment method") {
		t.Fatalf("two refund methods: err = %v", err)
	}
}

// A return from kashi settles on the sale's own method, On Account here; that
// was refused before ("on account cannot be used as change").
func TestAdminReturnOfOnAccountSaleSettlesOnTheAccount(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	if err := f.returnUnits(1, nil, f.onAccount.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 1000 {
		t.Fatalf("owed = %d, want 1000", got)
	}
	// A second return takes the rest; a third has nothing left to return.
	if err := f.returnUnits(1, nil, f.onAccount.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed = %d, want 0", got)
	}
}

// A sale paid in cash is refunded in cash as before.
func TestReturnOfCashSaleStaysCash(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	before := f.shiftBalance(t, f.cashbox.ID)
	items := fmt.Sprintf(`[{"code":%q,"barcode":%q,"qty":-1}]`, f.product.Code, f.product.Barcode)
	pays, _ := json.Marshal([]checkoutPayment{{AccountID: f.cashbox.ID, AmountCents: -1000, EntryType: "change"}})
	if _, err := f.app.checkoutReturnExchangeInvoice(items, string(pays), f.sourceID, f.salesperson.ID, "", 0, onAccountReturnPolicy); err != nil {
		t.Fatal(err)
	}
	if got := f.shiftBalance(t, f.cashbox.ID); got != before-1000 {
		t.Fatalf("cash = %d, want %d", got, before-1000)
	}
	if got := f.owed(t); got != 2000 {
		t.Fatalf("the on-account sale is untouched: owed = %d", got)
	}
}

// Migration 000141 moves an old cash refund of an unpaid on-account sale onto
// the account, giving the cash back to the drawer.
func TestMigrationMovesOldCashRefundOntoTheAccount(t *testing.T) {
	f := newOnAccountReturnFixture(t)
	ctx := context.Background()
	// The old way: a cash refund recorded straight in the tables.
	inv, err := f.store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: f.cashbox.ID, ShiftID: 1, InvoiceCode: "RS/CT/2026/00069", InvoiceIndex: 69, Year: 2026,
		ClientID: f.client.ID, InventoryID: 1, Subtotal: -2000, DiscountedTotal: -2000, GrandTotal: -2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		fmt.Sprintf(`INSERT INTO return_invoices (invoice_id, sales_invoice_id) VALUES (%d, %d)`, inv.ID, f.saleID),
		fmt.Sprintf(`INSERT INTO invoice_payments (invoice_id, cashbox_id, amount) VALUES (%d, %d, -2000)`, inv.ID, f.cashbox.ID),
		fmt.Sprintf(`INSERT INTO invoice_payment_entries (invoice_id, cashbox_id, amount, entry_type) VALUES (%d, %d, -2000, 'change')`, inv.ID, f.cashbox.ID),
		fmt.Sprintf(`INSERT INTO shifts_accounts_balances (account_id, shift_id, balance) VALUES (%d, 1, -2000)
			ON CONFLICT (account_id, shift_id) DO UPDATE SET balance = balance - 2000`, f.cashbox.ID),
	} {
		if _, err := f.database.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	cashBefore, onAccountBefore := f.shiftBalance(t, f.cashbox.ID), f.shiftBalance(t, f.onAccount.ID)
	if got := f.owed(t); got != 2000 {
		t.Fatalf("before the migration the debt stays: %d", got)
	}
	migration, err := os.ReadFile("db/migrations/000141_on_account_returns.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // running it twice changes nothing more
		if _, err := f.database.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed after migration = %d", got)
	}
	if got := f.shiftBalance(t, f.cashbox.ID); got != cashBefore+2000 {
		t.Fatalf("cash = %d, want %d", got, cashBefore+2000)
	}
	if got := f.shiftBalance(t, f.onAccount.ID); got != onAccountBefore-2000 {
		t.Fatalf("On Account = %d, want %d", got, onAccountBefore-2000)
	}
	var entryAccount int64
	if err := f.database.QueryRow(`SELECT cashbox_id FROM invoice_payment_entries WHERE invoice_id = ?`, inv.ID).Scan(&entryAccount); err != nil || entryAccount != f.onAccount.ID {
		t.Fatalf("payment entry account = %d, err %v", entryAccount, err)
	}
}
