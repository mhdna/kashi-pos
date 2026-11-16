package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

// ListOnAccountClients lists every client who still owes on account, biggest
// debt first, with what they were sold, what they paid back and what is left.
// All amounts are USD cents; fully paid clients are left out. yearMonth
// ("YYYY-MM") keeps only the sales rung up that month; "" lists every month.
func (a *App) ListOnAccountClients(yearMonth string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	// The month is the till's local month; created_at is stored in UTC, so
	// the bounds go in as UTC strings and compare as plain text.
	params := db.ListClientAccountBalancesParams{FromAt: "", ToAt: ""}
	if yearMonth != "" {
		start, err := time.ParseInLocation("2006-01", yearMonth, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid month %q; use YYYY-MM", yearMonth)
		}
		params.FromAt = start.UTC().Format("2006-01-02 15:04:05")
		params.ToAt = start.AddDate(0, 1, 0).UTC().Format("2006-01-02 15:04:05")
	}
	rows, err := a.store.ListClientAccountBalances(a.ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list on-account clients: %w", err)
	}
	clients := make([]map[string]any, 0, len(rows))
	var total, paid, outstanding int64
	for _, row := range rows {
		total += row.Total
		paid += row.Paid
		outstanding += row.Outstanding
		clients = append(clients, map[string]any{
			"id": row.ID, "name": row.Name, "phone": row.Phone,
			"invoiceCount": row.InvoiceCount, "total": row.Total, "paid": row.Paid,
			"outstanding": row.Outstanding, "lastSaleAt": row.LastSaleAt,
		})
	}
	return map[string]any{
		"clients": clients, "total": total, "paid": paid, "outstanding": outstanding,
	}, nil
}

// ListOnAccountMonths lists the months ("YYYY-MM", the till's local time)
// that have a sale still owed on account, newest first: the only months worth
// offering in the On Account page's month picker.
func (a *App) ListOnAccountMonths() ([]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	dates, err := a.store.ListOpenClientAccountEntryDates(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list on-account months: %w", err)
	}
	seen := map[string]bool{}
	months := make([]string, 0)
	for _, raw := range dates {
		at, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(raw), time.UTC)
		if err != nil {
			continue
		}
		month := at.In(time.Local).Format("2006-01")
		if !seen[month] {
			seen[month] = true
			months = append(months, month)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))
	return months, nil
}
