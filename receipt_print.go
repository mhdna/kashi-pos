package main

type printReceiptData struct {
	Items           []invoiceItem
	StoreName       string
	Branch          string
	Phone           string
	Salesperson     string
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
