package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

const (
	invoiceTypeSales    = "sales"
	invoiceTypeExchange = "return"
)

func (a *App) GetNextInvoiceCode(kind string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("database not initialized")
	}
	invoiceType, prefix, err := invoiceNumberingType(kind)
	if err != nil {
		return "", err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return "", fmt.Errorf("settings: %w", err)
	}
	branchCode, err := normalizeBranchCode(settings.BranchCode)
	if err != nil {
		return "", err
	}
	year := int64(time.Now().Year())
	index, err := a.store.GetInvoiceIndex(a.ctx, db.GetInvoiceIndexParams{
		Year: year,
		Type: invoiceType,
	})
	next := int64(1)
	if err == nil {
		next = index.LastIndex + 1
	} else if err != sql.ErrNoRows {
		return "", err
	}
	return formatInvoiceCode(prefix, branchCode, year, next), nil
}

func (a *App) allocateInvoiceCode(q *db.Queries, kind, branchCode string, year int64) (string, int64, error) {
	invoiceType, prefix, err := invoiceNumberingType(kind)
	if err != nil {
		return "", 0, err
	}
	branchCode, err = normalizeBranchCode(branchCode)
	if err != nil {
		return "", 0, err
	}
	index, err := q.IncrementInvoicesIndex(a.ctx, db.IncrementInvoicesIndexParams{
		Year: year,
		Type: invoiceType,
	})
	if err != nil {
		return "", 0, err
	}
	return formatInvoiceCode(prefix, branchCode, year, index), index, nil
}

func invoiceNumberingType(kind string) (invoiceType, prefix string, err error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "sales", "sale", "sa":
		return invoiceTypeSales, "SA", nil
	case "exchange", "return", "ex":
		if strings.ToLower(strings.TrimSpace(kind)) == "return" {
			return invoiceTypeExchange, "RS", nil
		}
		return invoiceTypeExchange, "EX", nil
	default:
		return "", "", fmt.Errorf("invalid invoice type %q", kind)
	}
}

func normalizeBranchCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || len(code) > 8 {
		return "", fmt.Errorf("branch code must contain 1 to 8 letters or numbers")
	}
	for _, character := range code {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return "", fmt.Errorf("branch code must contain only letters and numbers")
		}
	}
	return code, nil
}

func formatInvoiceCode(prefix, branchCode string, year, index int64) string {
	return fmt.Sprintf("%s/%s/%d/%05d", prefix, branchCode, year, index)
}
