package main

import (
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// GetClientAccountDetails returns the receivable balance, the invoices that
// created it, and every later payment applied to those invoices.
func (a *App) GetClientAccountDetails(clientID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if clientID <= 0 {
		return nil, fmt.Errorf("invalid client")
	}
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}
	entries, err := a.store.ListClientAccountEntries(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list client account invoices: %w", err)
	}
	payments, err := a.store.ListClientAccountPayments(a.ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list client account payments: %w", err)
	}

	entryList := make([]map[string]any, 0, len(entries))
	var balance int64
	for _, entry := range entries {
		outstanding := entry.Outstanding
		if outstanding < 0 {
			outstanding = 0
		}
		balance += outstanding
		entryList = append(entryList, map[string]any{
			"id": entry.ID, "invoiceId": entry.InvoiceID, "invoiceCode": entry.InvoiceCode,
			"amount": entry.Amount, "paid": entry.Paid, "outstanding": outstanding,
			"createdAt": entry.CreatedAt,
		})
	}
	paymentList := make([]map[string]any, 0, len(payments))
	for _, payment := range payments {
		paymentList = append(paymentList, map[string]any{
			"id": payment.ID, "invoiceId": payment.InvoiceID, "cashboxId": payment.CashboxID,
			"accountName": payment.AccountName, "currencyCode": payment.CurrencyCode,
			"shiftId": payment.ShiftID, "amount": payment.Amount, "createdAt": payment.CreatedAt,
		})
	}
	return map[string]any{
		"client": clientResponse(client), "balance": balance,
		"entries": entryList, "payments": paymentList,
	}, nil
}

// PayClientAccount applies collected money to one original on-account invoice.
// The receipt belongs to the currently open shift, not the sale's old shift.
func (a *App) PayClientAccount(invoiceID, cashboxID, amountCents int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if invoiceID <= 0 || cashboxID <= 0 || amountCents <= 0 {
		return nil, fmt.Errorf("invoice, payment method, and positive amount are required")
	}
	account, err := a.store.GetCashboxAccountByID(a.ctx, cashboxID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found")
	}
	if strings.EqualFold(strings.TrimSpace(account.Name), "On Account") ||
		strings.EqualFold(strings.TrimSpace(account.Name), "On Coupon") {
		return nil, fmt.Errorf("choose a collected payment method")
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("an open shift is required to receive payment")
	}

	var clientID int64
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		entry, entryErr := q.GetClientAccountEntryByInvoiceID(a.ctx, invoiceID)
		if entryErr != nil {
			return fmt.Errorf("on-account invoice not found")
		}
		paidValue, paidErr := q.GetClientAccountPaidByInvoiceID(a.ctx, invoiceID)
		if paidErr != nil {
			return fmt.Errorf("load prior payments: %w", paidErr)
		}
		outstanding := entry.Amount - databaseInt64(paidValue)
		if outstanding <= 0 {
			return fmt.Errorf("invoice is already paid")
		}
		if amountCents > outstanding {
			return fmt.Errorf("payment exceeds outstanding balance of %d cents", outstanding)
		}
		if _, createErr := q.CreateClientAccountPayment(a.ctx, db.CreateClientAccountPaymentParams{
			InvoiceID: invoiceID, CashboxID: cashboxID, ShiftID: shift.ID, Amount: amountCents,
		}); createErr != nil {
			return fmt.Errorf("save account payment: %w", createErr)
		}
		if _, balanceErr := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
			AccountID: cashboxID, ShiftID: shift.ID, Balance: amountCents,
		}); balanceErr != nil {
			return fmt.Errorf("add shift payment: %w", balanceErr)
		}
		clientID = entry.ClientID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.GetClientAccountDetails(clientID)
}
