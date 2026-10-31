package main

import (
	"database/sql"
	"testing"
)

// monthSalesShop is a till with a Cash and an On Account drawer account, a
// "calculate on checkout" coupon type and a walk-in client, with invoices
// written straight into the tables.
type monthSalesShop struct {
	t         *testing.T
	app       *App
	db        *sql.DB
	cash      int64
	onAccount int64
	shift     int64
	next      int64
}

func newMonthSalesShop(t *testing.T) *monthSalesShop {
	app, database := newProductTestApp(t)
	t.Cleanup(func() { database.Close() })
	s := &monthSalesShop{t: t, app: app, db: database}
	s.cash = s.insert(`INSERT INTO cashbox_accounts (name, currency_code) VALUES ('Cash test', 'USD')`)
	s.onAccount = s.lookupOrInsert(`SELECT id FROM cashbox_accounts WHERE lower(name) = 'on account'`,
		`INSERT INTO cashbox_accounts (name, currency_code) VALUES ('On Account', 'USD')`)
	// A shift opened on the last evening of September: its sales after
	// midnight belong to October.
	s.shift = s.insert(`INSERT INTO shifts (is_closed, created_at) VALUES (1, '2026-09-30 18:00:00')`)
	s.exec(`INSERT INTO coupon_types (name, value_cents, calculate_on_checkout) VALUES ('Voucher', 1000, 1)`)
	s.exec(`INSERT INTO coupons (code, status, type_id, value_cents) VALUES ('V1', 'redeemed', 1, 1000)`)
	s.lookupOrInsert(`SELECT id FROM clients LIMIT 1`, `INSERT INTO clients (name, phone) VALUES ('Walk-in', '00000000')`)
	s.lookupOrInsert(`SELECT id FROM inventories LIMIT 1`, `INSERT INTO inventories (name) VALUES ('Default')`)
	return s
}

func (s *monthSalesShop) exec(query string, args ...any) {
	s.t.Helper()
	if _, err := s.db.Exec(query, args...); err != nil {
		s.t.Fatalf("%s: %v", query, err)
	}
}

func (s *monthSalesShop) insert(query string, args ...any) int64 {
	s.t.Helper()
	res, err := s.db.Exec(query, args...)
	if err != nil {
		s.t.Fatalf("%s: %v", query, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func (s *monthSalesShop) lookupOrInsert(lookup, insert string) int64 {
	s.t.Helper()
	var id int64
	if err := s.db.QueryRow(lookup).Scan(&id); err == nil {
		return id
	}
	return s.insert(insert)
}

// invoice writes a sale (or, negative, a return) at a UTC time, paid to
// cashbox, and returns its id.
func (s *monthSalesShop) invoice(at string, grandTotal, cashbox int64) int64 {
	s.t.Helper()
	s.next++
	return s.insert(`INSERT INTO invoices (cashbox_id, shift_id, invoice_code, invoice_index, year, client_id, inventory_id,
		discount, subtotal, discounted_total, grand_total, created_at)
		VALUES (?, ?, ?, ?, 2026, (SELECT id FROM clients LIMIT 1), (SELECT id FROM inventories LIMIT 1), 0, ?, ?, ?, ?)`,
		cashbox, s.shift, "T-"+string(rune('A'+s.next)), s.next, grandTotal, grandTotal, grandTotal, at)
}

// The month's total counts sales by the day they were rung up, nets
// returns, leaves out a cancelled sale and its refund, and counts coupon
// and on-account sales in full while saying how much of each it holds.
func TestGetMonthSalesCountsLikeKashiReports(t *testing.T) {
	s := newMonthSalesShop(t)
	// Late September, in a shift opened that evening: not October.
	s.invoice("2026-09-30 19:00:00", 5000, s.cash)
	// After midnight in the same shift: October, though the shift isn't.
	s.invoice("2026-10-01 10:00:00", 10000, s.cash)
	// A return in October.
	s.invoice("2026-10-02 10:00:00", -2000, s.cash)
	// A sale on account, paid to the On Account account.
	onAccount := s.invoice("2026-10-03 10:00:00", 3000, s.cash)
	s.exec(`INSERT INTO invoice_payments (invoice_id, cashbox_id, amount) VALUES (?, ?, 3000)`, onAccount, s.onAccount)
	// A sale with no payment rows, booked whole to On Account.
	s.invoice("2026-10-04 10:00:00", 1500, s.onAccount)
	// A sale paid with a "calculate on checkout" coupon.
	coupon := s.invoice("2026-10-05 10:00:00", 1000, s.cash)
	s.exec(`INSERT INTO invoice_coupon_payments (invoice_id, coupon_code, coupon_type_id, amount) VALUES (?, 'V1', 1, 1000)`, coupon)
	// A sale cancelled in November: both it and its refund are left out.
	sale := s.invoice("2026-10-06 10:00:00", 9900, s.cash)
	refund := s.invoice("2026-11-01 10:00:00", -9900, s.cash)
	s.exec(`UPDATE invoices SET cancelled_at = '2026-11-01 10:00:00' WHERE id IN (?, ?)`, sale, refund)

	got, err := s.app.GetMonthSales("2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(10000 - 2000 + 3000 + 1500 + 1000); got.Total != want {
		t.Errorf("total = %d, want %d", got.Total, want)
	}
	if got.OnAccount != 4500 {
		t.Errorf("on account = %d, want 4500", got.OnAccount)
	}
	if got.DeferredCoupons != 1000 {
		t.Errorf("coupons = %d, want 1000", got.DeferredCoupons)
	}
	var daysTotal int64
	for _, d := range got.Days {
		daysTotal += d.Total
	}
	if daysTotal != got.Total {
		t.Errorf("days add up to %d, total is %d", daysTotal, got.Total)
	}

	november, err := s.app.GetMonthSales("2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if november.Total != 0 {
		t.Errorf("November total = %d, want 0 (the cancelled sale's refund is left out)", november.Total)
	}
	if _, err := s.app.GetMonthSales("October"); err == nil {
		t.Error("a malformed month was accepted")
	}
}
