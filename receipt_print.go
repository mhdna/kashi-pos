package main

import "strings"

type printReceiptData struct {
	Items            []invoiceItem
	StoreName        string
	Branch           string
	Phone            string
	BranchPhone      string
	Description      string
	RegisterName     string
	Salesperson      string
	Customer         string
	Notes            string
	ReturnPolicy     string
	Website          string
	Instagram        string
	SocialPlatforms  []string
	SocialHandles    map[string]string
	Font             string
	TitleSize        float64
	BodySize         float64
	BodyFont         string
	Cutoff           bool
	ReceiptNo        string
	PrinterName      string
	GrandTotalCents  int64
	TaxCents         int64
	Payments         []receiptPayment
	RoundingCurrency receiptCurrency
	RoundingMode     string
	ReceiptWidth     int64
	ReceiptHeight    int64
	ShiftReport      *shiftReportData
}

// receiptCurrency carries the fields a receipt needs to display an amount
// in a given currency (resolved from the currencies table at build time),
// so printing code never hardcodes a currency literal.
type receiptCurrency struct {
	Code              string
	Symbol            string
	UnitsPerUsdMicros int64
	CashRoundingUnit  int64
}

type receiptPayment struct {
	AccountID    int64
	AccountName  string
	CurrencyCode string
	Currency     receiptCurrency
	AmountCents  int64
	EntryType    string
}

func aggregateReceiptPayments(payments []receiptPayment) []receiptPayment {
	type paymentKey struct {
		accountID    int64
		currencyCode string
		entryType    string
	}

	aggregated := make([]receiptPayment, 0, len(payments))
	indexByKey := make(map[paymentKey]int, len(payments))
	for _, payment := range payments {
		key := paymentKey{
			accountID:    payment.AccountID,
			currencyCode: payment.CurrencyCode,
			entryType:    payment.EntryType,
		}
		if index, ok := indexByKey[key]; ok {
			aggregated[index].AmountCents += payment.AmountCents
			continue
		}
		indexByKey[key] = len(aggregated)
		aggregated = append(aggregated, payment)
	}
	return aggregated
}

func formatReceiptSalesperson(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "Not specified"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	last := []rune(parts[len(parts)-1])
	if len(last) == 0 {
		return parts[0]
	}
	return parts[0] + " " + string(last[0]) + "."
}

func receiptInvoiceLabel(receiptNo string) string {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(receiptNo)), "EX/") {
		return "Exchange Invoice"
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(receiptNo)), "RS/") {
		return "Return Invoice"
	}
	return "Sales Invoice"
}
