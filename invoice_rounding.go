package main

import (
	"database/sql"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// RoundInvoice applies the current configured rounding rule to an existing
// invoice and keeps its payment, payment-entry, client-account, and shift
// balance records in sync. The UI exposes this operation to administrators.
func (a *App) RoundInvoice(invoiceID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if invoiceID <= 0 {
		return fmt.Errorf("invalid invoice")
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	invoice, err := a.store.GetInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	targetTotal, err := roundCentsByCurrency(invoice.GrandTotal, settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
	if err != nil {
		return fmt.Errorf("round invoice total: %w", err)
	}

	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		payments, err := q.ListInvoicePayments(a.ctx, invoiceID)
		if err != nil {
			return err
		}
		entries, err := q.ListInvoicePaymentEntries(a.ctx, invoiceID)
		if err != nil {
			return err
		}

		newPayments := make(map[int64]int64, len(payments))
		var paymentTotal int64
		firstPayment := int64(0)
		for _, payment := range payments {
			amount := payment.Amount
			account, accountErr := q.GetCashboxAccountByID(a.ctx, payment.CashboxID)
			if accountErr != nil {
				return accountErr
			}
			if amount > 0 && strings.EqualFold(account.CurrencyCode, "LBP") && isCashAccount(account.Name, account.CurrencyCode) {
				amount, err = roundCashEquivalentCents(amount, settings.ExchangeRate, settings.RoundingMode)
				if err != nil {
					return fmt.Errorf("round LBP payment: %w", err)
				}
			}
			newPayments[payment.CashboxID] = amount
			paymentTotal += amount
			if firstPayment == 0 && amount > 0 {
				firstPayment = payment.CashboxID
			}
		}
		if firstPayment != 0 {
			newPayments[firstPayment] += targetTotal - paymentTotal
			if newPayments[firstPayment] <= 0 {
				return fmt.Errorf("rounded invoice payment would be invalid")
			}
		}

		for _, payment := range payments {
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: payment.CashboxID, ShiftID: invoice.ShiftID, Balance: -payment.Amount}); err != nil {
				return err
			}
		}
		for cashboxID, amount := range newPayments {
			if err := q.UpdateInvoicePaymentAmount(a.ctx, db.UpdateInvoicePaymentAmountParams{Amount: amount, InvoiceID: invoiceID, CashboxID: cashboxID}); err != nil {
				return err
			}
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: cashboxID, ShiftID: invoice.ShiftID, Balance: amount}); err != nil {
				return err
			}
		}

		newEntries := make([]int64, len(entries))
		var entryTotal int64
		firstEntry := -1
		for i, entry := range entries {
			amount := entry.Amount
			account, accountErr := q.GetCashboxAccountByID(a.ctx, entry.CashboxID)
			if accountErr != nil {
				return accountErr
			}
			if amount > 0 && strings.EqualFold(account.CurrencyCode, "LBP") && isCashAccount(account.Name, account.CurrencyCode) {
				amount, err = roundCashEquivalentCents(amount, settings.ExchangeRate, settings.RoundingMode)
				if err != nil {
					return fmt.Errorf("round LBP payment entry: %w", err)
				}
			}
			newEntries[i] = amount
			entryTotal += amount
			if firstEntry < 0 && amount > 0 && entry.EntryType == "payment" {
				firstEntry = i
			}
		}
		if firstEntry >= 0 {
			newEntries[firstEntry] += targetTotal - entryTotal
			if newEntries[firstEntry] == 0 {
				return fmt.Errorf("rounded payment entry would be invalid")
			}
		}
		for i, entry := range entries {
			if err := q.UpdateInvoicePaymentEntryAmount(a.ctx, db.UpdateInvoicePaymentEntryAmountParams{Amount: newEntries[i], ID: entry.ID}); err != nil {
				return err
			}
		}

		if len(payments) == 0 {
			if _, accountErr := q.GetClientAccountEntryByInvoiceID(a.ctx, invoiceID); accountErr == nil {
				if targetTotal <= 0 {
					return fmt.Errorf("rounded client account amount would be invalid")
				}
				if err := q.UpdateClientAccountEntryAmount(a.ctx, db.UpdateClientAccountEntryAmountParams{Amount: targetTotal, InvoiceID: invoiceID}); err != nil {
					return err
				}
			} else if accountErr != sql.ErrNoRows {
				return accountErr
			}
		}
		return q.UpdateInvoiceGrandTotal(a.ctx, db.UpdateInvoiceGrandTotalParams{GrandTotal: targetTotal, ID: invoiceID})
	})
}
