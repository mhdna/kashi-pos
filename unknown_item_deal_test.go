package main

import (
	"encoding/json"
	"testing"
)

// unknownLine is an unknown item line as the cart sends it: one unit at
// priceCents, with the salesperson's deal choice ("" for the default).
func unknownLine(ref string, priceCents int64, choice string) salesInvoiceItem {
	return salesInvoiceItem{
		Code: "UNKNOWN", Barcode: "UNKNOWN", Qty: 1, UnitPricesCents: []int64{priceCents}, UnitDiscounts: []int64{0},
		UnknownDeal: choice, LineRef: ref,
	}
}

func quoteItems(t *testing.T, app *App, items ...salesInvoiceItem) InvoiceQuote {
	t.Helper()
	itemsJSON, _ := json.Marshal(items)
	quote, err := app.QuoteInvoiceTotal(string(itemsJSON))
	if err != nil {
		t.Fatal(err)
	}
	return quote
}

func unknownQuoteLines(quote InvoiceQuote) []LineQuote {
	var out []LineQuote
	for _, l := range quote.Lines {
		if l.Barcode == "UNKNOWN" {
			out = append(out, l)
		}
	}
	return out
}

// With no choice made, an unknown item is on the shop's "Buy 2 get 1": with
// shirts A ($100) and B ($80) it is the cheapest of three, so it's free. Its
// quote line names the choice and echoes the cart's line ref.
func TestUnknownItemJoinsBuy2Get1ByDefault(t *testing.T) {
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	quote := quoteItems(t, shop.app,
		salesInvoiceItem{Barcode: "bc-A", Qty: 1}, salesInvoiceItem{Barcode: "bc-B", Qty: 1},
		unknownLine("u1", 3000, ""))
	if quote.GrandTotalCents != 18000 {
		t.Fatalf("total = %d, want 18000 (the unknown item free)", quote.GrandTotalCents)
	}
	lines := unknownQuoteLines(quote)
	if len(lines) != 1 || !lines[0].Free || lines[0].UnknownDeal != unknownDealDefault || lines[0].LineRef != "u1" {
		t.Fatalf("unknown quote lines = %+v", lines)
	}
}

// Outside a set, the unit gets the fallback the salesperson picked: 30% or
// 20%. "none" takes it out of the deal, so it neither gets one nor makes a
// set with the shirts.
func TestUnknownItemDealChoice(t *testing.T) {
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		choice string
		want   int64
	}{{unknownDeal30, 2100}, {unknownDeal20, 2400}, {unknownDealNone, 3000}} {
		quote := quoteItems(t, shop.app, unknownLine("u1", 3000, tc.choice))
		if quote.GrandTotalCents != tc.want {
			t.Errorf("%s: total = %d, want %d", tc.choice, quote.GrandTotalCents, tc.want)
		}
		if lines := unknownQuoteLines(quote); len(lines) != 1 || lines[0].UnknownDeal != tc.choice {
			t.Errorf("%s: unknown quote lines = %+v", tc.choice, lines)
		}
	}

	quote := quoteItems(t, shop.app,
		salesInvoiceItem{Barcode: "bc-A", Qty: 1}, salesInvoiceItem{Barcode: "bc-B", Qty: 1},
		unknownLine("u1", 3000, unknownDealNone))
	if quote.GrandTotalCents != 18000+3000 {
		t.Fatalf("total with the deal off = %d, want 21000", quote.GrandTotalCents)
	}
}

// Two unknown items on different choices pool together but keep their own
// fallback, and the sale goes through checkout with the deal recorded.
func TestUnknownItemsCheckoutOnTheDeal(t *testing.T) {
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	items := []salesInvoiceItem{
		{Barcode: "bc-A", Qty: 1}, {Barcode: "bc-B", Qty: 1},
		unknownLine("u1", 3000, unknownDeal30), unknownLine("u2", 4000, unknownDeal20),
	}
	quote := quoteItems(t, shop.app, items...)
	// Units $100, $80, $40, $30: one set of three, the $30 free; the $100
	// left over is a shirt with the deal's own fallback (none).
	if quote.GrandTotalCents != 10000+8000+4000 {
		t.Fatalf("total = %d, want 22000", quote.GrandTotalCents)
	}
	itemsJSON, _ := json.Marshal(items)
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	shop.app.unknownSessionRef = "" // no picture session: call the tx directly
	sale, err := shop.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: shop.client.ID})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := shop.store.ListInvoiceProducts(shop.app.ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var freeUnknown bool
	for _, l := range lines {
		if l.UnitPrice == 3000 && l.DealUid == "deal-b2g1" && l.LineTotal == 0 {
			freeUnknown = true
		}
	}
	if !freeUnknown {
		t.Fatalf("no free unknown line recorded: %+v", lines)
	}
}

// A typed discount keeps the unknown item out of the deal: a deal never
// stacks on another saving.
func TestUnknownItemWithADiscountStaysOffTheDeal(t *testing.T) {
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	line := unknownLine("u1", 3000, unknownDeal30)
	line.UnitDiscounts = []int64{10}
	quote := quoteItems(t, shop.app, line)
	if quote.GrandTotalCents != 2700 {
		t.Fatalf("total = %d, want 2700 (10%% off, no deal)", quote.GrandTotalCents)
	}
	if lines := unknownQuoteLines(quote); len(lines) != 1 || lines[0].UnknownDeal != unknownDealNone || lines[0].Deal != "" {
		t.Fatalf("unknown quote lines = %+v", lines)
	}
}

func TestUnknownItemDealRejectsOtherChoices(t *testing.T) {
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	itemsJSON, _ := json.Marshal([]salesInvoiceItem{unknownLine("u1", 3000, "b2g1_50")})
	if _, err := shop.app.QuoteInvoiceTotal(string(itemsJSON)); err == nil {
		t.Fatal("a choice outside the three was accepted")
	}
}

// A till with no "Buy 2 get 1" sells unknown items at their price, and says
// it has no deal to offer them.
func TestUnknownItemWithoutABuy2Get1Deal(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	listPricing, err := loadActivePricing(app.ctx, app.store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	items := []salesInvoiceItem{unknownLine("u1", 3000, unknownDeal30)}
	products, _, discounted, _, err := app.validateAndPriceItems(app.store.Queries, 0, items, "half_up", false, listPricing, false)
	if err != nil {
		t.Fatal(err)
	}
	if discounted != 3000 || products[0].unknownDeal != "" || products[0].dealUID != "" {
		t.Fatalf("discounted %d, line %+v", discounted, products[0])
	}
}

// The pick is the Buy 2 get 1 most products are on; other deal kinds, and
// "Buy 2 get 1 50% off", don't count.
func TestPickUnknownItemDeal(t *testing.T) {
	deals := map[string]dealTerms{
		"a":    {UID: "a", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100},
		"b":    {UID: "b", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100},
		"half": {UID: "half", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 50},
		"vol":  {UID: "vol", Kind: dealVolumePercent, BuyQty: 3, Percent: 20},
	}
	discounts := map[string]activeDiscount{
		"p1": {dealUID: "b"}, "p2": {dealUID: "b"}, "p3": {dealUID: "a"},
		"p4": {dealUID: "half"}, "p5": {dealUID: "half"}, "p6": {dealUID: "half"},
	}
	if got := pickUnknownItemDeal(deals, discounts); got != "b" {
		t.Fatalf("picked %q, want b", got)
	}
	if got := pickUnknownItemDeal(deals, nil); got != "a" {
		t.Fatalf("picked %q with no rows, want a (lowest uid)", got)
	}
	if got := pickUnknownItemDeal(map[string]dealTerms{"vol": deals["vol"]}, discounts); got != "" {
		t.Fatalf("picked %q from no Buy 2 get 1, want none", got)
	}
}
