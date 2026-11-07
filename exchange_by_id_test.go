package main

import (
	"encoding/json"
	"testing"
)

// The exchange screen's "Returned items" loads the sale by id: the same
// answer as by its code, what is left after earlier returns, and an error
// for a sale that isn't one.
func TestGetExchangeInvoiceByID(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C", "SOCKS")

	byID, err := shop.app.GetExchangeInvoiceByID(sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	byCode, err := shop.app.GetExchangeInvoice(sale.InvoiceCode)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(byID)
	b, _ := json.Marshal(byCode)
	if string(a) != string(b) {
		t.Fatalf("by id %s\nby code %s", a, b)
	}

	// After SOCKS is exchanged, it is no longer offered.
	shop.exchange(t, sale.InvoiceID, []string{"SOCKS"}, []string{"SOCKS"})
	left, err := shop.app.GetExchangeInvoiceByID(sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range left["items"].([]map[string]any) {
		if item["code"] == "SOCKS" {
			t.Fatalf("returned socks still offered: %+v", item)
		}
	}

	if _, err := shop.app.GetExchangeInvoiceByID(999999); err == nil {
		t.Fatal("a sale that doesn't exist loaded")
	}
}
