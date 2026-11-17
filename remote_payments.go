package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// remotePayment is one payment line central's admin entered for a remote
// return, exchange or settlement change: the till's payment method by name
// (central lists the same methods), its amount in USD cents, signed (negative
// is money given back), and for a cash-rounded currency (LBP) the amount as
// counted in its own units.
type remotePayment struct {
	AccountName string `json:"account_name"`
	AmountCents int64  `json:"amount_cents"`
	AmountUnits int64  `json:"amount_units,omitempty"`
}

// remotePaymentsJSON turns central's payment lines into the checkout payments
// this till's calculator would send. "" when there are none, which keeps the
// old behaviour (the till settles on its own).
func (a *App) remotePaymentsJSON(ctx context.Context, lines []remotePayment) (string, error) {
	if len(lines) == 0 {
		return "", nil
	}
	accounts, err := a.store.ListCashboxAccounts(ctx)
	if err != nil {
		return "", fmt.Errorf("payment methods: %w", err)
	}
	byName := make(map[string]db.CashboxAccount, len(accounts))
	for _, account := range accounts {
		byName[strings.ToLower(strings.TrimSpace(account.Name))] = account
	}
	payments := make([]checkoutPayment, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line.AccountName)
		account, ok := byName[strings.ToLower(name)]
		if !ok {
			return "", fmt.Errorf("this till has no payment method %q", name)
		}
		if strings.EqualFold(strings.TrimSpace(account.Name), "On Coupon") {
			return "", fmt.Errorf("coupons can't be entered from central; do it on the till")
		}
		if line.AmountCents == 0 {
			return "", fmt.Errorf("%s has no amount", name)
		}
		entryType := "payment"
		if line.AmountCents < 0 {
			entryType = "change"
		}
		units := line.AmountUnits
		if units < 0 {
			units = -units
		}
		payments = append(payments, checkoutPayment{
			AccountID: account.ID, AmountCents: line.AmountCents, EntryType: entryType, AmountUnits: units,
		})
	}
	raw, err := json.Marshal(payments)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// checkExactPayments is central's payments against the till's own total: the
// admin worked from an estimate, so a different total is refused with the
// till's figure rather than settled some other way. A cash-rounded entry (LBP
// counted in units) is left to validatePayments' rounding tolerance.
func checkExactPayments(payments []checkoutPayment, total int64) error {
	var sum int64
	for _, p := range payments {
		if p.AmountUnits != 0 {
			return nil
		}
		sum += p.AmountCents
	}
	if sum != total {
		return fmt.Errorf("the till's total is %s but the payments entered add up to %s; enter payments for %s",
			formatCents(total), formatCents(sum), formatCents(total))
	}
	return nil
}

// changeInvoiceSettlementPayload is central's "Change settlement" of a sale
// (command change_invoice_settlement): the sale by its client_ref and its new
// split between payment methods, which must add up to its total.
type changeInvoiceSettlementPayload struct {
	ClientRef string          `json:"client_ref"`
	Payments  []remotePayment `json:"payments"`
	Reason    string          `json:"reason"`
	EditedBy  string          `json:"edited_by"`
}

func (a *App) executeChangeInvoiceSettlementCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	var p changeInvoiceSettlementPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode change_invoice_settlement payload: %w", err)
	}
	ref := strings.TrimSpace(p.ClientRef)
	if ref == "" {
		return fmt.Errorf("client_ref is required")
	}
	if len(p.Payments) == 0 {
		return fmt.Errorf("enter at least one payment")
	}
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, ref)
	if err != nil {
		return fmt.Errorf("sale not found for client_ref %q: %w", ref, err)
	}
	invoice, err := a.store.GetSalesInvoiceByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("only a sale's settlement can be changed")
	}
	if invoice.CancelledAt.Valid {
		return fmt.Errorf("this sale was cancelled")
	}
	old, err := a.store.ListInvoicePayments(ctx, invoiceID)
	if err != nil {
		return err
	}
	for _, payment := range old {
		account, err := a.store.GetCashboxAccountByID(ctx, payment.CashboxID)
		if err == nil && strings.EqualFold(strings.TrimSpace(account.Name), "On Coupon") {
			return fmt.Errorf("this sale was paid with a coupon; change its settlement on the till")
		}
	}
	paymentsJSON, err := a.remotePaymentsJSON(ctx, p.Payments)
	if err != nil {
		return err
	}
	var payments []checkoutPayment
	if err := json.Unmarshal([]byte(paymentsJSON), &payments); err != nil {
		return err
	}
	if err := checkExactPayments(payments, invoice.GrandTotal); err != nil {
		return err
	}
	_, err = a.SettleInvoicePayments(invoiceID, paymentsJSON)
	return err
}
