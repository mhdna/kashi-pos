package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// onAccountRefund is how much of a return's refund comes off what the client
// still owes On Account on the sale: no money changes hands for what was never
// paid, so that part becomes a negative On Account entry, which the balance
// (client_account_lines) nets off the debt.
type onAccountRefund struct {
	entry *checkoutPayment // the On Account entry to add once the rest is checked
	rest  int64            // what is still refunded the way the cashier chose (<= 0)
}

// settleRefundOnAccount splits a return's refund (total < 0) between the
// client's debt and the cashier's payments. It gives back the payments that
// remain to check against rest: none when the debt takes the whole refund,
// else the one refund method the cashier chose, cut to rest. An On Account
// entry the cashier (or an admin return, which settles on the sale's own
// method) put in is the debt's part and is dropped from them.
func settleRefundOnAccount(ctx context.Context, q *db.Queries, saleID, total int64, payments []checkoutPayment) ([]checkoutPayment, onAccountRefund, error) {
	none := onAccountRefund{rest: total}
	if total >= 0 {
		return payments, none, nil
	}
	owed, err := q.GetClientAccountOwedByInvoiceID(ctx, saleID)
	if errors.Is(err, sql.ErrNoRows) {
		return payments, none, nil // not an on-account sale
	}
	if err != nil {
		return nil, none, fmt.Errorf("on-account balance: %w", err)
	}
	if owed <= 0 {
		return payments, none, nil
	}
	onAccount, err := onAccountMethod(ctx, q)
	if err != nil {
		return nil, none, err
	}
	credit := min(-total, owed)
	settled := onAccountRefund{
		entry: &checkoutPayment{AccountID: onAccount.ID, AmountCents: -credit, EntryType: "change"},
		rest:  total + credit,
	}
	if settled.rest == 0 {
		return nil, settled, nil
	}
	var others []checkoutPayment
	for _, p := range payments {
		if p.AccountID != onAccount.ID {
			others = append(others, p)
		}
	}
	if len(others) != 1 || others[0].AmountCents >= 0 {
		return nil, none, fmt.Errorf("%s of this refund comes off what the client owes on account; refund the other %s with one payment method",
			formatCents(credit), formatCents(-settled.rest))
	}
	refund := others[0]
	refund.AmountCents = settled.rest
	refund.AmountUnits = 0
	return []checkoutPayment{refund}, settled, nil
}

func onAccountMethod(ctx context.Context, q *db.Queries) (db.CashboxAccount, error) {
	accounts, err := q.ListCashboxAccounts(ctx)
	if err != nil {
		return db.CashboxAccount{}, fmt.Errorf("payment methods: %w", err)
	}
	for _, account := range accounts {
		if strings.EqualFold(strings.TrimSpace(account.Name), "On Account") {
			return account, nil
		}
	}
	return db.CashboxAccount{}, fmt.Errorf("no On Account payment method to settle the return against")
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

// resettleClientAccount brings a sale's on-account entry in line with its new
// settlement, and gives back the payments recordSpecialPaymentMetadata should
// still record. Before, the old entry stayed: a sale moved from On Account to
// cash still showed as owed, and moving it to On Account again failed.
//   - Nothing paid back yet: the old entry goes, and the new On Account amount
//     (if any) is recorded afresh.
//   - Some paid back: the entry keeps its payments and takes the new amount,
//     which can't be less than what was paid.
func resettleClientAccount(ctx context.Context, q *db.Queries, invoiceID int64, payments []checkoutPayment) ([]checkoutPayment, error) {
	entry, err := q.GetClientAccountEntryByInvoiceID(ctx, invoiceID)
	if errors.Is(err, sql.ErrNoRows) {
		return payments, nil
	}
	if err != nil {
		return nil, fmt.Errorf("on-account entry: %w", err)
	}
	paidValue, err := q.GetClientAccountPaidByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("on-account payments: %w", err)
	}
	paid := databaseInt64(paidValue)
	if paid == 0 {
		if err := q.DeleteClientAccountEntryByInvoiceID(ctx, invoiceID); err != nil {
			return nil, fmt.Errorf("clear on-account entry: %w", err)
		}
		return payments, nil
	}
	var onAccount int64
	others := make([]checkoutPayment, 0, len(payments))
	for _, p := range payments {
		account, err := q.GetCashboxAccountByID(ctx, p.AccountID)
		if err != nil {
			return nil, fmt.Errorf("payment account: %w", err)
		}
		if strings.EqualFold(strings.TrimSpace(account.Name), "On Account") {
			onAccount += p.AmountCents
			continue
		}
		others = append(others, p)
	}
	if onAccount < paid {
		return nil, fmt.Errorf("the client already paid %s of this sale back; keep at least that much On Account", formatCents(paid))
	}
	if err := q.UpdateClientAccountEntryAmount(ctx, db.UpdateClientAccountEntryAmountParams{Amount: onAccount, InvoiceID: entry.InvoiceID}); err != nil {
		return nil, fmt.Errorf("update on-account entry: %w", err)
	}
	return others, nil
}
