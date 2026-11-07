package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// The frontend checks real quotes from here (frontend/src/lib/
// exchangeQuote.test.ts reads this file), so a change to what the backend
// sends that the screen can't add up fails there. Rewrite it with
// KASHI_UPDATE_GOLDEN=1 go test -run TestExchangeQuoteGolden .
const exchangeQuoteGoldenPath = "frontend/src/lib/testdata/exchange_quotes.json"

type goldenCartLine struct {
	Code    string `json:"code"`
	Barcode string `json:"barcode,omitempty"`
	Qty     int64  `json:"qty"`
}

type goldenExchange struct {
	Name  string           `json:"name"`
	Cart  []goldenCartLine `json:"cart"`
	Quote ExchangeQuote    `json:"quote"`
}

func TestExchangeQuoteGolden(t *testing.T) {
	var golden []goldenExchange
	add := func(name string, shop dealShop, saleID int64, lines []exchangeLine) ExchangeQuote {
		t.Helper()
		quote := quoteExchangeLines(t, shop, saleID, lines)
		var cart []goldenCartLine
		for _, l := range lines {
			cart = append(cart, goldenCartLine{l.Code, l.Barcode, l.Qty})
		}
		golden = append(golden, goldenExchange{name, cart, quote})
		return quote
	}

	{
		shop := setUpDealShop(t)
		unknown, err := ensureUnknownProduct(shop.app.ctx, shop.store.Queries, shop.app.unknownBarcode())
		if err != nil {
			t.Fatal(err)
		}
		sale := shop.sell(t, "A", "B", "C")
		add("paid deal unit for an unknown item", shop, sale.InvoiceID, []exchangeLine{
			{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{12000}}, line("B", -1),
		})
		add("free unit for cheaper socks", shop, sale.InvoiceID, []exchangeLine{line("C", -1), line("SOCKS", 1)})
		add("paid unit for a jacket off the deal", shop, sale.InvoiceID, []exchangeLine{line("A", -1), line("JACKET", 1)})
		add("not enough new items yet", shop, sale.InvoiceID, []exchangeLine{line("A", -1), line("SOCKS", 1)})
		add("everything back for jackets", shop, sale.InvoiceID, []exchangeLine{line("A", -1), line("B", -1), line("C", -1), line("JACKET", 2)})
	}
	{
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "C", "C")
		add("alike units on two lines", shop, sale.InvoiceID, []exchangeLine{line("C", -1), line("C", -1), line("JACKET", 1)})
	}
	{
		shop := setUpDealShopWithFallback(t, 20)
		sale := shop.sell(t, "A", "B", "C")
		add("fallback percent after a return", shop, sale.InvoiceID, []exchangeLine{line("A", -1), line("JACKET", 1)})
	}
	{
		shop := setUpDealShop(t)
		sale := shop.sell(t, "SOCKS")
		setExchangeTaxAndRounding(t, shop, 0.11, "LBP")
		add("tax and cash rounding", shop, sale.InvoiceID, []exchangeLine{line("SOCKS", -1), line("A", 1)})
	}

	got, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("frontend/src/lib/testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exchangeQuoteGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(exchangeQuoteGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the backend's exchange quotes changed; check the screen still adds them up and rewrite %s with KASHI_UPDATE_GOLDEN=1:\n%s", exchangeQuoteGoldenPath, got)
	}
}
