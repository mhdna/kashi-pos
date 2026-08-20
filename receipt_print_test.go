package main

import (
	"reflect"
	"testing"
)

func TestAggregateReceiptPaymentsByAccountCurrencyAndEntryType(t *testing.T) {
	payments := []receiptPayment{
		{AccountID: 1, AccountName: "Cash", CurrencyCode: "LBP", Currency: testLBPReceiptCurrency, AmountCents: -1111, EntryType: "change"},
		{AccountID: 1, AccountName: "Cash", CurrencyCode: "LBP", Currency: testLBPReceiptCurrency, AmountCents: -389, EntryType: "change"},
		{AccountID: 2, AccountName: "Cash", CurrencyCode: "USD", AmountCents: 2000, EntryType: "payment"},
		{AccountID: 2, AccountName: "Cash", CurrencyCode: "USD", AmountCents: 800, EntryType: "payment"},
		{AccountID: 3, AccountName: "Whish", CurrencyCode: "USD", AmountCents: 500, EntryType: "payment"},
		{AccountID: 1, AccountName: "Cash", CurrencyCode: "LBP", Currency: testLBPReceiptCurrency, AmountCents: 200, EntryType: "payment"},
	}

	got := aggregateReceiptPayments(payments)
	want := []receiptPayment{
		{AccountID: 1, AccountName: "Cash", CurrencyCode: "LBP", Currency: testLBPReceiptCurrency, AmountCents: -1500, EntryType: "change"},
		{AccountID: 2, AccountName: "Cash", CurrencyCode: "USD", AmountCents: 2800, EntryType: "payment"},
		{AccountID: 3, AccountName: "Whish", CurrencyCode: "USD", AmountCents: 500, EntryType: "payment"},
		{AccountID: 1, AccountName: "Cash", CurrencyCode: "LBP", Currency: testLBPReceiptCurrency, AmountCents: 200, EntryType: "payment"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aggregateReceiptPayments() = %#v, want %#v", got, want)
	}
	if formatted := formatReceiptPayment(got[0], roundingHalfUp); formatted != "1,350,000 L.L." {
		t.Fatalf("formatted aggregated LBP change = %q, want %q", formatted, "1,350,000 L.L.")
	}
}
