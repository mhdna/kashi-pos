package main

import "time"

import "strings"

type printReceiptData struct {
	Items           []invoiceItem
	StoreName       string
	Branch          string
	Phone           string
	BranchPhone     string
	Description     string
	RegisterName    string
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
	TaxCents        int64
	// The sale's invoice discount: a row of its own, apart from the item
	// discounts (Items are priced before it).
	InvoiceDiscount invoiceDiscount
	// The customer's referral code and the line explaining it (referrals.go);
	// both empty when it isn't printed.
	ReferralCode string
	ReferralNote string
	// The branch's wording and the parts it leaves out (receipt_texts.go).
	Texts            receiptTexts
	Payments         []receiptPayment
	RoundingCurrency receiptCurrency
	RoundingMode     string
	ReceiptWidth     int64
	ReceiptHeight    int64
	ShiftReport      *shiftReportData
	// Theme and Layout pick the receipt's look (receipt_theme.go); empty
	// means the default.
	Theme  string
	Layout string
	// Now, when set, is the date the receipt prints (previews pin it); else
	// the clock's.
	Now time.Time
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

// formatReceiptSalesperson is the salesman as a receipt names him: his
// first name only ("Rana Haddad" prints "Rana").
func formatReceiptSalesperson(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "Not specified"
	}
	return parts[0]
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
