package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	db "kashi-pos/db/sqlc"
)

// A one-time repair of three Centro (CT) sales from 2026-11-21. The
// "Independence Day $50" deal was meant to come off once per $100, but the
// till took it off only once, so the cashier gave the missing $50s as "On
// Coupon" payments ($150 over the three sales) and one "On Account" $50.
// Central's copies were corrected by hand; this gives the till's copies the
// full deal too: the invoice discount re-spread over the lines, the totals
// lowered by the same, and the coupon and on-account payments taken out of
// the sale, its shift's balances, the coupon and the client's account.
//
// Each sale is touched only while it is still exactly as sold, so any other
// till, or this one after the repair, does nothing. Nothing is sent to
// central, which already has the corrected sales.

const independenceDayRepairFlag = "repair_independence_day_2026_11_21"

type independenceDayRepair struct {
	clientRef string
	discount  int64 // the deal in full: $50 for every $100
}

var independenceDayRepairs = []independenceDayRepair{
	{"161bd4ac-f52b-430b-924d-a326c507dca3", 10000}, // SA/CT/2026/03406
	{"8e12d0e8-f0a7-40bb-a241-c4eebaf64097", 15000}, // SA/CT/2026/03408
	{"7eb6348e-3ae5-4ad6-b75e-15b077223c95", 10000}, // SA/CT/2026/03409
}

const independenceDayDealName = "Independence Day $50"

// repairIndependenceDaySales runs the repair once per till; a sale that
// can't be repaired is logged and left as it was.
func (a *App) repairIndependenceDaySales(ctx context.Context) {
	if a.store == nil || a.conn == nil || a.flagEnabled(ctx, independenceDayRepairFlag) {
		return
	}
	for _, r := range independenceDayRepairs {
		if fixed, err := a.repairIndependenceDaySale(ctx, r); err != nil {
			log.Printf("independence day repair %s: %v", r.clientRef, err)
		} else if fixed {
			log.Printf("independence day repair %s: done", r.clientRef)
		}
	}
	if err := a.setFeatureFlagBool(ctx, independenceDayRepairFlag, true); err != nil {
		log.Println("independence day repair flag:", err)
	}
}

func isCouponOrAccount(name string) bool {
	name = strings.TrimSpace(name)
	return strings.EqualFold(name, "On Coupon") || strings.EqualFold(name, "On Account")
}

func (a *App) repairIndependenceDaySale(ctx context.Context, r independenceDayRepair) (bool, error) {
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, r.clientRef)
	if err == sql.ErrNoRows {
		return false, nil // not this till's sale
	}
	if err != nil {
		return false, err
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return false, err
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return false, err
	}
	tx, err := a.conn.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	q := db.New(tx)

	inv, err := q.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return false, err
	}
	extra := r.discount - inv.InvoiceDiscount
	if inv.CancelledAt.Valid || inv.InvoiceDiscountName != independenceDayDealName || inv.InvoiceDiscount != 5000 || extra <= 0 {
		return false, nil // already repaired, or not as sold
	}

	// The coupon and on-account payments must be exactly the missing deal.
	payments, err := q.ListInvoicePayments(ctx, invoiceID)
	if err != nil {
		return false, err
	}
	special := map[int64]bool{}
	var specialSum int64
	for _, p := range payments {
		account, err := q.GetCashboxAccountByID(ctx, p.CashboxID)
		if err != nil {
			return false, err
		}
		if isCouponOrAccount(account.Name) {
			special[p.CashboxID] = true
			specialSum += p.Amount
		}
	}
	if specialSum != extra {
		return false, fmt.Errorf("coupon and on-account payments are %d, the missing deal is %d; left as it was", specialSum, extra)
	}
	if paid, err := q.GetClientAccountPaidByInvoiceID(ctx, invoiceID); err != nil {
		return false, err
	} else if databaseInt64(paid) != 0 {
		return false, fmt.Errorf("part of the on-account amount was already paid back; left as it was")
	}

	// The deal in full, spread over the lines as checkout spreads it.
	lines, err := q.ListInvoiceProducts(ctx, invoiceID)
	if err != nil {
		return false, err
	}
	slices.SortFunc(lines, func(x, y db.InvoiceProduct) int { return int(x.ID - y.ID) })
	bases := make([]int64, len(lines))
	var sum int64
	dearest := -1
	for i, l := range lines {
		bases[i] = l.LineTotal + l.InvoiceDiscount
		if bases[i] <= 0 {
			continue
		}
		sum += bases[i]
		if dearest < 0 || bases[i] > bases[dearest] {
			dearest = i
		}
	}
	if dearest < 0 || r.discount > sum {
		return false, fmt.Errorf("the lines can't carry %d off", r.discount)
	}
	shares := make([]int64, len(lines))
	var given int64
	for i := range lines {
		if bases[i] > 0 {
			shares[i] = r.discount * bases[i] / sum
			given += shares[i]
		}
	}
	shares[dearest] += r.discount - given
	for i, l := range lines {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_products SET invoice_discount = ?, line_total = ? WHERE id = ?`,
			shares[i], bases[i]-shares[i], l.ID); err != nil {
			return false, err
		}
	}
	grandTotal := inv.GrandTotal - extra
	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET invoice_discount = ?, discounted_total = ?, grand_total = ? WHERE id = ?`,
		r.discount, inv.DiscountedTotal-extra, grandTotal, invoiceID); err != nil {
		return false, err
	}
	if inv.LbpTotal.Valid {
		if _, _, err := stampInvoiceLBPTotal(ctx, q, invoiceID, grandTotal, roundingMode); err != nil {
			return false, err
		}
	}

	// The coupon and on-account payments leave the sale and its shift.
	for _, p := range payments {
		if !special[p.CashboxID] || p.Amount == 0 {
			continue
		}
		if _, err := q.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{
			AccountID: p.CashboxID, ShiftID: inv.ShiftID, Balance: -p.Amount,
		}); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_payments WHERE invoice_id = ? AND cashbox_id = ?`, invoiceID, p.CashboxID); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_payment_entries WHERE invoice_id = ? AND cashbox_id = ?`, invoiceID, p.CashboxID); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_coupon_payments WHERE invoice_id = ?`, invoiceID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE coupons SET status = 'active', redeemed_invoice_id = NULL WHERE redeemed_invoice_id = ?`, invoiceID); err != nil {
		return false, err
	}
	if err := q.DeleteClientAccountEntryByInvoiceID(ctx, invoiceID); err != nil {
		return false, err
	}

	// What's left must add up: lines to the invoice, payments to its total.
	var lineSum, shareSum, paidSum int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(line_total), 0), COALESCE(SUM(invoice_discount), 0) FROM invoice_products WHERE invoice_id = ?`, invoiceID).Scan(&lineSum, &shareSum); err != nil {
		return false, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM invoice_payments WHERE invoice_id = ?`, invoiceID).Scan(&paidSum); err != nil {
		return false, err
	}
	if lineSum != inv.DiscountedTotal-extra || shareSum != r.discount || paidSum != grandTotal {
		return false, fmt.Errorf("totals don't add up after the repair (lines %d, shares %d, paid %d, total %d); left as it was", lineSum, shareSum, paidSum, grandTotal)
	}
	return true, tx.Commit()
}
