package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	db "kashi-pos/db/sqlc"
)

// repair_deal_sales: central's command for sales where a deal the till
// couldn't give was paid "On Coupon" or "On Account" instead (Centro's
// Independence Day, 2026-11-21: the till took $50 off once, not once per
// $100, and the cashier made up the rest). For each sale it gives the deal
// in full: the invoice discount set to the full amount and re-spread over
// the lines, the totals lowered by what it adds, and that much of coupon
// and on-account payment taken out of the sale, its shift's balances, the
// coupon and the client's account.
//
// A sale is changed only while its coupon and on-account payments are
// exactly what the deal adds, so a repeat of the command, or a sale settled
// some other way since, is left as it is. Nothing is sent to central: it
// sends the command with its own copies corrected. The command fails,
// naming each sale and why, when any sale couldn't be repaired, so central
// sees it.

type repairDealSalesPayload struct {
	// Name is the deal's name, as the invoice discount shows it.
	Name  string `json:"name"`
	Sales []struct {
		ClientRef string `json:"client_ref"`
		// Discount is the deal in full on that sale, in cents.
		Discount int64 `json:"discount"`
	} `json:"sales"`
}

func (a *App) executeRepairDealSalesCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil || a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	var p repairDealSalesPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode repair_deal_sales payload: %w", err)
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Sales) == 0 {
		return fmt.Errorf("a deal name and at least one sale are required")
	}
	var problems []string
	for _, s := range p.Sales {
		if err := a.repairDealSale(ctx, strings.TrimSpace(s.ClientRef), s.Discount, p.Name); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", s.ClientRef, err))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func isCouponOrAccount(name string) bool {
	name = strings.TrimSpace(name)
	return strings.EqualFold(name, "On Coupon") || strings.EqualFold(name, "On Account")
}

// errDealAlreadyRepaired: the sale already has the deal in full.
var errDealAlreadyRepaired = errors.New("already has the deal in full")

// repairDealSale gives one sale the deal in full; nil when it did, or when
// it already had it.
func (a *App) repairDealSale(ctx context.Context, clientRef string, discount int64, name string) error {
	if err := a.repairDealSaleTx(ctx, clientRef, discount, name); !errors.Is(err, errDealAlreadyRepaired) {
		return err
	}
	return nil
}

func (a *App) repairDealSaleTx(ctx context.Context, clientRef string, discount int64, name string) error {
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, clientRef)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("no such sale on this till")
	}
	if err != nil {
		return err
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return err
	}
	tx, err := a.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := db.New(tx)

	inv, err := q.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv.CancelledAt.Valid {
		return errors.New("the sale was cancelled")
	}
	if inv.InvoiceDiscount == discount && inv.InvoiceDiscountName == name {
		return errDealAlreadyRepaired
	}
	// Either no invoice discount yet, or this deal taken off short.
	if inv.InvoiceDiscount != 0 && inv.InvoiceDiscountName != name {
		return fmt.Errorf("it has another invoice discount, %q", inv.InvoiceDiscountName)
	}
	extra := discount - inv.InvoiceDiscount
	if extra <= 0 {
		return fmt.Errorf("it already has %s off, not less than %s", formatCents(inv.InvoiceDiscount), formatCents(discount))
	}

	// The coupon and on-account payments must be exactly what the deal adds.
	payments, err := q.ListInvoicePayments(ctx, invoiceID)
	if err != nil {
		return err
	}
	special := map[int64]bool{}
	var specialSum int64
	for _, p := range payments {
		account, err := q.GetCashboxAccountByID(ctx, p.CashboxID)
		if err != nil {
			return err
		}
		if isCouponOrAccount(account.Name) {
			special[p.CashboxID] = true
			specialSum += p.Amount
		}
	}
	if specialSum != extra {
		return fmt.Errorf("its coupon and on-account payments are %s, the deal adds %s", formatCents(specialSum), formatCents(extra))
	}
	if paid, err := q.GetClientAccountPaidByInvoiceID(ctx, invoiceID); err != nil {
		return err
	} else if databaseInt64(paid) != 0 {
		return errors.New("part of its on-account amount was already paid back")
	}

	// The deal in full, spread over the lines as checkout spreads it.
	lines, err := q.ListInvoiceProducts(ctx, invoiceID)
	if err != nil {
		return err
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
	if dearest < 0 || discount > sum {
		return fmt.Errorf("its lines come to %s, less than %s off", formatCents(sum), formatCents(discount))
	}
	shares := make([]int64, len(lines))
	var given int64
	for i := range lines {
		if bases[i] > 0 {
			shares[i] = discount * bases[i] / sum
			given += shares[i]
		}
	}
	shares[dearest] += discount - given
	for i, l := range lines {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_products SET invoice_discount = ?, line_total = ? WHERE id = ?`,
			shares[i], bases[i]-shares[i], l.ID); err != nil {
			return err
		}
	}
	grandTotal := inv.GrandTotal - extra
	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET invoice_discount = ?, invoice_discount_name = ?, discounted_total = ?, grand_total = ? WHERE id = ?`,
		discount, name, inv.DiscountedTotal-extra, grandTotal, invoiceID); err != nil {
		return err
	}
	if inv.LbpTotal.Valid {
		if _, _, err := stampInvoiceLBPTotal(ctx, q, invoiceID, grandTotal, roundingMode); err != nil {
			return err
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
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_payments WHERE invoice_id = ? AND cashbox_id = ?`, invoiceID, p.CashboxID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_payment_entries WHERE invoice_id = ? AND cashbox_id = ?`, invoiceID, p.CashboxID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_coupon_payments WHERE invoice_id = ?`, invoiceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE coupons SET status = 'active', redeemed_invoice_id = NULL WHERE redeemed_invoice_id = ?`, invoiceID); err != nil {
		return err
	}
	if err := q.DeleteClientAccountEntryByInvoiceID(ctx, invoiceID); err != nil {
		return err
	}

	// What's left must add up: lines to the invoice, payments to its total.
	var lineSum, shareSum, paidSum int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(line_total), 0), COALESCE(SUM(invoice_discount), 0) FROM invoice_products WHERE invoice_id = ?`, invoiceID).Scan(&lineSum, &shareSum); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM invoice_payments WHERE invoice_id = ?`, invoiceID).Scan(&paidSum); err != nil {
		return err
	}
	if lineSum != inv.DiscountedTotal-extra || shareSum != discount || paidSum != grandTotal {
		return fmt.Errorf("totals wouldn't add up (lines %d, shares %d, paid %d, total %d), left as it was", lineSum, shareSum, paidSum, grandTotal)
	}
	return tx.Commit()
}
