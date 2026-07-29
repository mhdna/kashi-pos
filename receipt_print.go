package main

import "strings"

type printReceiptData struct {
	Items           []invoiceItem
	StoreName       string
	Branch          string
	Phone           string
	BranchPhone     string
	Description     string
	CounterNumber   string
	Salesperson     string
	Customer        string
	Notes           string
	ReturnPolicy    string
	Website         string
	Instagram       string
	SocialPlatforms []string
	SocialHandles   map[string]string
	Font            string
	TitleSize       float64
	BodySize        float64
	BodyFont        string
	Cutoff          bool
	ReceiptNo       string
	PrinterName     string
	GrandTotalCents int64
	Payments        []receiptPayment
	ExchangeRate    int64
	RoundingMode    string
	ReceiptWidth    int64
	ReceiptHeight   int64
}

type receiptPayment struct {
	AccountName  string
	CurrencyCode string
	AmountCents  int64
	EntryType    string
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
	return "Sales Invoice"
}
