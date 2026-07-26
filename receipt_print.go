package main

type printReceiptData struct {
	Items           []invoiceItem
	StoreName       string
	Branch          string
	Phone           string
	Description     string
	Salesperson     string
	Customer        string
	ReturnPolicy    string
	Website         string
	Instagram       string
	Font            string
	TitleSize       float64
	BodySize        float64
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
