package main

import (
	"bytes"
	"testing"
)

func TestBuildESCPosReceiptContainsInvoiceAndAuthoritativeTotals(t *testing.T) {
	payload := buildESCPosReceipt(printReceiptData{
		StoreName:       "Kashi Store",
		Branch:          "CN",
		ReceiptNo:       "SA/CN/2026/00001",
		Items:           []invoiceItem{{Name: "Shirt", Price: 10, Discount: 10, Qty: 2}},
		GrandTotalCents: 1980,
		ExchangeRate:    90000,
		ReceiptWidth:    80,
	})

	for _, want := range []string{"Kashi Store", "SA/CN/2026/00001", "Shirt", "$19.80", "TOTAL", "LBP TOTAL"} {
		if !bytes.Contains(payload, []byte(want)) {
			t.Fatalf("payload does not contain %q: %q", want, payload)
		}
	}
	if !bytes.HasPrefix(payload, []byte{esc, '@'}) {
		t.Fatalf("payload does not begin with ESC/POS initialize command")
	}
	if !bytes.HasSuffix(payload, []byte{gs, cutMode, 3}) {
		t.Fatalf("payload does not end with ESC/POS cut command")
	}
}

func TestBuildESCPosReceiptContainsTenderAndPolicy(t *testing.T) {
	payload := buildESCPosReceipt(printReceiptData{
		StoreName:       "Kashi Store",
		ReceiptNo:       "INV-1",
		GrandTotalCents: 1000,
		ExchangeRate:    90000,
		RoundingMode:    roundingHalfUp,
		Payments: []receiptPayment{
			{AccountName: "Cash", CurrencyCode: "USD", AmountCents: 2000, EntryType: "payment"},
			{AccountName: "Cash", CurrencyCode: "LBP", AmountCents: -1000, EntryType: "change"},
		},
	})
	for _, want := range []string{"CASH RECEIVED Cash", "CHANGE RETURNED Cash", "PAYMENT DETAILS", "RETURN & EXCHANGE POLICY", "3 days"} {
		if !bytes.Contains(payload, []byte(want)) {
			t.Fatalf("payload does not contain %q: %q", want, payload)
		}
	}
}

func TestBuildESCPosReceiptUses58mmColumnsAndSanitizesText(t *testing.T) {
	payload := buildESCPosReceipt(printReceiptData{
		StoreName:       "Café",
		ReceiptNo:       "INV-1",
		Items:           []invoiceItem{{Name: "Very long product name that must be shortened", Price: 1, Qty: 1}},
		GrandTotalCents: 100,
		ReceiptWidth:    58,
	})

	if bytes.Contains(payload, []byte("é")) {
		t.Fatal("payload contains unsupported non-ASCII text")
	}
	if !bytes.Contains(payload, []byte("Caf?")) {
		t.Fatalf("payload does not sanitize non-ASCII text: %q", payload)
	}
}

func TestBuildESCPosReceiptIncludesOnlyNonEmptyNote(t *testing.T) {
	withNote := buildESCPosReceipt(printReceiptData{
		StoreName: "Kashi Store",
		ReceiptNo: "INV-1",
		Notes:     "Customer requested gift wrapping",
	})
	if !bytes.Contains(withNote, []byte("Note: Customer requested gift wrapping")) {
		t.Fatal("receipt does not contain the invoice note")
	}

	withoutNote := buildESCPosReceipt(printReceiptData{
		StoreName: "Kashi Store",
		ReceiptNo: "INV-1",
		Notes:     "   ",
	})
	if bytes.Contains(withoutNote, []byte("Note:")) {
		t.Fatal("receipt contains an empty invoice note")
	}
}
