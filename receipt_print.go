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
	ExchangeRate    int64
	ReceiptWidth    int64
	ReceiptHeight   int64
}
