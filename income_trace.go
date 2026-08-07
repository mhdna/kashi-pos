package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type incomeTraceContribution struct {
	Kind               string `json:"kind"`
	Timestamp          string `json:"timestamp"`
	AmountCents        int64  `json:"amountCents"`
	InvoiceID          int64  `json:"invoiceId,omitempty"`
	InvoiceCode        string `json:"invoiceCode,omitempty"`
	ShiftID            int64  `json:"shiftId,omitempty"`
	EntryType          string `json:"entryType,omitempty"`
	SourceAccount      string `json:"sourceAccount,omitempty"`
	DestinationAccount string `json:"destinationAccount,omitempty"`
}

type incomeTraceResult struct {
	Date          string                    `json:"date"`
	AccountID     int64                     `json:"accountId"`
	AccountName   string                    `json:"accountName"`
	CurrencyCode  string                    `json:"currencyCode"`
	TotalCents    int64                     `json:"totalCents"`
	Contributions []incomeTraceContribution `json:"contributions"`
}

func (a *App) incomeTrace(ctx context.Context, rawDate, accountName string) (incomeTraceResult, error) {
	if a.conn == nil {
		return incomeTraceResult{}, fmt.Errorf("database not initialized")
	}
	date, err := traceDate(rawDate)
	if err != nil {
		return incomeTraceResult{}, err
	}
	if strings.TrimSpace(accountName) == "" {
		accountName = "Whish"
	}

	result := incomeTraceResult{Date: date, Contributions: []incomeTraceContribution{}}
	if err := a.conn.QueryRowContext(ctx, `
		SELECT id, name, currency_code
		FROM cashbox_accounts
		WHERE lower(trim(name)) = lower(trim(?))
		LIMIT 1`, accountName).Scan(&result.AccountID, &result.AccountName, &result.CurrencyCode); err != nil {
		if err == sql.ErrNoRows {
			return incomeTraceResult{}, fmt.Errorf("cashbox account %q not found", accountName)
		}
		return incomeTraceResult{}, err
	}

	rows, err := a.conn.QueryContext(ctx, `
		SELECT 'invoice-payment', i.created_at, ip.amount, i.id, i.invoice_code, i.shift_id,
		       COALESCE((SELECT group_concat(entry_type, ',')
		                 FROM invoice_payment_entries ipe
		                 WHERE ipe.invoice_id = i.id AND ipe.cashbox_id = ip.cashbox_id), ''), '', ''
		FROM invoices i
		JOIN invoice_payments ip ON ip.invoice_id = i.id
		WHERE ip.cashbox_id = ? AND date(i.created_at, 'localtime') = ?
		UNION ALL
		SELECT 'invoice-default', i.created_at, i.grand_total, i.id, i.invoice_code, i.shift_id, 'payment', '', ''
		FROM invoices i
		WHERE i.cashbox_id = ? AND date(i.created_at, 'localtime') = ?
		  AND NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
		UNION ALL
		SELECT 'client-account-payment', cap.created_at, cap.amount, cap.invoice_id, '', cap.shift_id, 'payment', '', ''
		FROM client_account_payments cap
		WHERE cap.cashbox_id = ? AND date(cap.created_at, 'localtime') = ?
		UNION ALL
		SELECT 'balance-swap', sbr.created_at, -sbr.amount, 0, '', sbr.shift_id, 'balance swap', src.name, dst.name
		FROM shift_balance_replacements sbr
		JOIN cashbox_accounts src ON src.id = sbr.from_cashbox_id
		JOIN cashbox_accounts dst ON dst.id = sbr.to_cashbox_id
		WHERE sbr.from_cashbox_id = ? AND date(sbr.created_at, 'localtime') = ?
		UNION ALL
		SELECT 'balance-swap', sbr.created_at, sbr.amount, 0, '', sbr.shift_id, 'balance swap', src.name, dst.name
		FROM shift_balance_replacements sbr
		JOIN cashbox_accounts src ON src.id = sbr.from_cashbox_id
		JOIN cashbox_accounts dst ON dst.id = sbr.to_cashbox_id
		WHERE sbr.to_cashbox_id = ? AND date(sbr.created_at, 'localtime') = ?
		ORDER BY 2, 1`,
		result.AccountID, date, result.AccountID, date, result.AccountID, date,
		result.AccountID, date, result.AccountID, date)
	if err != nil {
		return incomeTraceResult{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item incomeTraceContribution
		if err := rows.Scan(&item.Kind, &item.Timestamp, &item.AmountCents, &item.InvoiceID, &item.InvoiceCode, &item.ShiftID, &item.EntryType, &item.SourceAccount, &item.DestinationAccount); err != nil {
			return incomeTraceResult{}, err
		}
		result.Contributions = append(result.Contributions, item)
		result.TotalCents += item.AmountCents
	}
	if err := rows.Err(); err != nil {
		return incomeTraceResult{}, err
	}

	return result, nil
}

func traceDate(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid date %q; use YYYY-MM-DD", raw)
	}
	return date.Format("2006-01-02"), nil
}
