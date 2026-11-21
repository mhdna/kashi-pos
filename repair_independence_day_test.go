package main

import (
	"context"
	"testing"

	db "kashi-pos/db/sqlc"
)

// rigIndependenceDaySale makes a sale the way the Centro till made it: the
// deal off once, the missing $50s paid "On Coupon" (and "On Account"), and
// the sale carrying one of the repair's client refs.
func rigIndependenceDaySale(t *testing.T, shop dealShop, ref string, coupon, account int64, codes ...string) int64 {
	t.Helper()
	ctx := context.Background()
	sale, _ := shop.sellPicked(t, []string{"indep"}, codes...)
	if sale.InvoiceDiscount.Cents != 5000 {
		t.Fatalf("sold with %+v", sale.InvoiceDiscount)
	}
	id := sale.InvoiceID
	inv, err := shop.store.GetInvoiceByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := shop.store.ListCashboxAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int64{}
	for _, a := range accounts {
		byName[a.Name] = a.ID
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := shop.app.conn.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE invoice_client_refs SET client_ref = ? WHERE invoice_id = ?`, ref, id)
	old, err := shop.store.ListInvoicePayments(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range old {
		if _, err := shop.store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{AccountID: p.CashboxID, ShiftID: inv.ShiftID, Balance: -p.Amount}); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM invoice_payments WHERE invoice_id = ?`, id)
	exec(`DELETE FROM invoice_payment_entries WHERE invoice_id = ?`, id)
	cash := inv.GrandTotal - coupon - account
	pay := func(accountID, amount int64) {
		exec(`INSERT INTO invoice_payments (invoice_id, cashbox_id, amount) VALUES (?, ?, ?)`, id, accountID, amount)
		exec(`INSERT INTO invoice_payment_entries (invoice_id, cashbox_id, amount, entry_type) VALUES (?, ?, ?, 'payment')`, id, accountID, amount)
		if _, err := shop.store.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{AccountID: accountID, ShiftID: inv.ShiftID, Balance: amount}); err != nil {
			t.Fatal(err)
		}
	}
	pay(shop.cashbox.ID, cash)
	if coupon > 0 {
		pay(byName["On Coupon"], coupon)
		exec(`INSERT OR IGNORE INTO coupon_types (id, name, value_cents, currency_code, is_active, reference_only, calculate_on_checkout, code_optional) VALUES (900, 'Fifty', 5000, 'USD', 1, 0, 0, 1)`)
		exec(`INSERT INTO invoice_coupon_payments (invoice_id, coupon_code, coupon_type_id, amount) VALUES (?, '', 900, ?)`, id, coupon)
	}
	if account > 0 {
		pay(byName["On Account"], account)
		exec(`INSERT INTO client_account_entries (client_id, invoice_id, amount) VALUES (?, ?, ?)`, inv.ClientID, id, account)
	}
	return id
}

func shiftBalance(t *testing.T, shop dealShop, name string) int64 {
	t.Helper()
	var v int64
	if err := shop.app.conn.QueryRow(`SELECT COALESCE(SUM(b.balance), 0) FROM shifts_accounts_balances b JOIN cashbox_accounts a ON a.id = b.account_id WHERE a.name = ?`, name).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// The reported case: the Centro till's shift shows On Coupon $150 and $50
// on account for the Independence Day sales. After the update the deal is
// in full on those sales and both are gone; cash is untouched.
func TestRepairIndependenceDaySales(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	shop.app.seedPlaceholderCashboxAccounts()
	once := independenceDay
	once.Repeats = false
	addDiscount(t, shop, once)

	// $220: 2 x $50 is due, $50 came off, $50 went On Coupon.
	a := rigIndependenceDaySale(t, shop, independenceDayRepairs[0].clientRef, 5000, 0, "JACKET", "JACKET", "SOCKS")
	// $300: 3 x $50 due, $50 off, $50 On Coupon and $50 On Account.
	b := rigIndependenceDaySale(t, shop, independenceDayRepairs[1].clientRef, 5000, 5000, "JACKET", "JACKET", "JACKET")
	// An ordinary sale with the deal: not touched.
	plain, _ := shop.sellPicked(t, []string{"indep"}, "JACKET", "SOCKS")
	cashBefore := shiftBalance(t, shop, "Cash")

	shop.app.repairIndependenceDaySales(ctx)

	for _, c := range []struct {
		id, disc, total int64
	}{{a, 10000, 12000}, {b, 15000, 15000}} {
		inv, err := shop.store.GetInvoiceByID(ctx, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if inv.InvoiceDiscount != c.disc || inv.GrandTotal != c.total || inv.DiscountedTotal != c.total {
			t.Fatalf("invoice %d = discount %d, total %d / %d", c.id, inv.InvoiceDiscount, inv.GrandTotal, inv.DiscountedTotal)
		}
		lines, _ := shop.store.ListInvoiceProducts(ctx, c.id)
		var shares, totals int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
			totals += l.LineTotal
			if l.LineTotal < 0 {
				t.Fatalf("negative line %+v", l)
			}
		}
		pays, _ := shop.store.ListInvoicePayments(ctx, c.id)
		var paid int64
		for _, p := range pays {
			paid += p.Amount
			if p.CashboxID != shop.cashbox.ID {
				t.Fatalf("payment left on %d", p.CashboxID)
			}
		}
		if shares != c.disc || totals != c.total || paid != c.total {
			t.Fatalf("invoice %d: shares %d, lines %d, paid %d", c.id, shares, totals, paid)
		}
	}
	if got := shiftBalance(t, shop, "On Coupon"); got != 0 {
		t.Fatalf("On Coupon still %d", got)
	}
	if got := shiftBalance(t, shop, "On Account"); got != 0 {
		t.Fatalf("On Account still %d", got)
	}
	if got := shiftBalance(t, shop, "Cash"); got != cashBefore {
		t.Fatalf("cash %d, was %d", got, cashBefore)
	}
	var entries, coupons int64
	shop.app.conn.QueryRow(`SELECT COUNT(*) FROM client_account_entries`).Scan(&entries)
	shop.app.conn.QueryRow(`SELECT COUNT(*) FROM invoice_coupon_payments`).Scan(&coupons)
	if entries != 0 || coupons != 0 {
		t.Fatalf("left %d on-account entries, %d coupon payments", entries, coupons)
	}
	if inv, _ := shop.store.GetInvoiceByID(ctx, plain.InvoiceID); inv.InvoiceDiscount != 5000 {
		t.Fatalf("ordinary sale changed: %+v", inv)
	}

	// Once only: a second start changes nothing.
	shop.app.repairIndependenceDaySales(ctx)
	if inv, _ := shop.store.GetInvoiceByID(ctx, a); inv.InvoiceDiscount != 10000 || inv.GrandTotal != 12000 {
		t.Fatalf("second run = %+v", inv)
	}
}

// A sale that isn't as sold (a different payment split) is left alone.
func TestRepairIndependenceDayLeavesOddSalesAlone(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	shop.app.seedPlaceholderCashboxAccounts()
	once := independenceDay
	once.Repeats = false
	addDiscount(t, shop, once)
	// $50 is due on top but only $20 went On Coupon.
	id := rigIndependenceDaySale(t, shop, independenceDayRepairs[0].clientRef, 2000, 0, "JACKET", "JACKET", "SOCKS")
	shop.app.repairIndependenceDaySales(ctx)
	inv, _ := shop.store.GetInvoiceByID(ctx, id)
	if inv.InvoiceDiscount != 5000 || inv.GrandTotal != 17000 {
		t.Fatalf("odd sale changed: %+v", inv)
	}
	if got := shiftBalance(t, shop, "On Coupon"); got != 2000 {
		t.Fatalf("On Coupon %d", got)
	}
}
