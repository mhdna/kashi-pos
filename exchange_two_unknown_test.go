package main

import (
	"encoding/json"
	"math/rand"
	"sort"
	"testing"
)

// The reported case: an exchange selling two "Unknown item" replacements at
// different typed prices ($13.30 and $24.50) was refused with "send a
// custom-priced replacement "UNKNOWN" as a single line": both lines carry the
// UNKNOWN code, so the till tried to merge them into one. Each line with its
// own price is now its own replacement.
func TestExchangeSellsTwoUnknownItemsAtTheirOwnPrices(t *testing.T) {
	app, sourceID, salesperson, replacement, cashbox := newExchangeReplacementPricingFixture(t)
	unknown, err := ensureUnknownProduct(app.ctx, app.store.Queries, app.unknownBarcode())
	if err != nil {
		t.Fatal(err)
	}
	items, _ := json.Marshal([]exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: "RETURN-P", Qty: -1},
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{1330}},
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{2450}},
		{Code: replacement.Code, Qty: 1},
	})

	quote, err := app.QuoteExchangeTotal(string(items), sourceID)
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if quote.ReturnedTotalCents != 2000 || quote.ReplacementTotalCents != 1330+2450+500 {
		t.Fatalf("quote returned %d replacements %d, want 2000 and 4280", quote.ReturnedTotalCents, quote.ReplacementTotalCents)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := app.checkoutReturnExchangeInvoice(string(items), string(payments), sourceID, salesperson.ID, "", 0, posExchangePolicy)
	if err != nil {
		t.Fatalf("exchange with two unknown items: %v", err)
	}
	if code, _ := result["invoiceCode"].(string); code == "" {
		t.Fatalf("no invoice code in %v", result)
	}
	// The fixture's sale is invoice 1; the exchange is the next one.
	lines, err := app.store.ListInvoiceProducts(app.ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	var unknownPrices []int64
	var sold, returned, total int64
	for _, line := range lines {
		total += line.LineTotal
		switch {
		case line.Quantity < 0:
			returned += -line.Quantity
		case line.ProductID == unknown.ID:
			unknownPrices = append(unknownPrices, line.UnitPrice)
			sold += line.Quantity
		default:
			sold += line.Quantity
		}
	}
	sort.Slice(unknownPrices, func(i, j int) bool { return unknownPrices[i] < unknownPrices[j] })
	if len(unknownPrices) != 2 || unknownPrices[0] != 1330 || unknownPrices[1] != 2450 {
		t.Fatalf("unknown lines at %v, want two lines at 1330 and 2450", unknownPrices)
	}
	if sold != 3 || returned != 2 {
		t.Fatalf("sold %d returned %d, want 3 and 2", sold, returned)
	}
	// What was stored is what the quote said.
	if total != quote.ReplacementTotalCents-quote.ReturnedTotalCents {
		t.Fatalf("stored lines add to %d, quote said %d", total, quote.ReplacementTotalCents-quote.ReturnedTotalCents)
	}
}

// Plain lines of one product still merge into one line; a priced line of the
// same product stays beside them.
func TestExchangeMergesOnlyListPricedLines(t *testing.T) {
	app, _, _, replacement, _ := newExchangeReplacementPricingFixture(t)
	unknown, err := ensureUnknownProduct(app.ctx, app.store.Queries, app.unknownBarcode())
	if err != nil {
		t.Fatal(err)
	}
	request, err := parseExchangeRequest(mustJSON(t, []exchangeLine{
		{Code: "RETURN-P", Qty: -1},
		{Code: replacement.Code, Qty: 1},
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{900}},
		{Code: replacement.Code, Qty: 2},
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{900}},
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{}
	for _, r := range request.replacements {
		got = append(got, r.Qty)
	}
	// REPLACE-P 1+2 merged; each priced unknown line on its own; the
	// unpriced one after them on its own (and later plain ones would join it).
	if len(request.replacements) != 4 || got[0] != 3 || got[1] != 1 || got[2] != 1 || got[3] != 1 {
		t.Fatalf("replacements %+v", request.replacements)
	}
	// Every line the cashier sent still points at the group it went into.
	var qty int64
	for _, s := range request.sources {
		if !s.returned {
			qty += s.qty
		}
	}
	if qty != 6 {
		t.Fatalf("sources add to %d replacement units, want 6", qty)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Any exchange cart the screen can build goes through: random mixes of
// unknown items at their own prices (one or several, the same price or not),
// list-priced products on one line or several, in any order. Each is quoted,
// checked out, and what is stored must be what the quote said, line for line.
// The till must never refuse a cart like this.
func TestExchangeAcceptsAnyCartTheScreenBuilds(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	for trial := 0; trial < 40; trial++ {
		app, sourceID, salesperson, replacement, cashbox := newExchangeReplacementPricingFixture(t)
		unknown, err := ensureUnknownProduct(app.ctx, app.store.Queries, app.unknownBarcode())
		if err != nil {
			t.Fatal(err)
		}
		returnedUnits := 1 + rng.Intn(3) // the fixture sold 3 at $10
		var lines []exchangeLine
		for i := 0; i < returnedUnits; i++ {
			lines = append(lines, exchangeLine{Code: "RETURN-P", Qty: -1})
		}
		var want int64
		prices := []int64{}
		for n := 1 + rng.Intn(6); n > 0; n-- {
			if rng.Intn(2) == 0 {
				price := int64(100 + rng.Intn(5000))
				if rng.Intn(4) == 0 && len(prices) > 0 {
					price = prices[rng.Intn(len(prices))] // two unknown items at one price
				}
				prices = append(prices, price)
				lines = append(lines, exchangeLine{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{price}})
				want += price
			} else {
				qty := int64(1 + rng.Intn(2))
				lines = append(lines, exchangeLine{Code: replacement.Code, Qty: qty})
				want += 500 * qty
			}
		}
		// The replacements must at least cover what comes back.
		if short := int64(1000*returnedUnits) - want; short > 0 {
			lines = append(lines, exchangeLine{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{short}})
			prices = append(prices, short)
			want += short
		}
		rng.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
		items := mustJSON(t, lines)

		quote, err := app.QuoteExchangeTotal(items, sourceID)
		if err != nil {
			t.Fatalf("trial %d: the screen's cart was refused at quote: %v\n%s", trial, err, items)
		}
		if quote.ReplacementTotalCents != want || quote.ReturnedTotalCents != int64(1000*returnedUnits) {
			t.Fatalf("trial %d: quote replacements %d returned %d, want %d and %d", trial, quote.ReplacementTotalCents, quote.ReturnedTotalCents, want, 1000*returnedUnits)
		}
		var sum int64
		for _, l := range quote.Lines {
			sum += l.LineTotalCents
		}
		if sum != quote.NetCents {
			t.Fatalf("trial %d: quote lines add to %d, net %d", trial, sum, quote.NetCents)
		}
		payments := "[]"
		if quote.GrandTotalCents > 0 {
			payments = mustJSON(t, []checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
		}
		if _, err := app.checkoutReturnExchangeInvoice(items, payments, sourceID, salesperson.ID, "", 0, posExchangePolicy); err != nil {
			t.Fatalf("trial %d: the screen's cart was refused at checkout: %v\n%s", trial, err, items)
		}
		stored, err := app.store.ListInvoiceProducts(app.ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		gotPrices := []int64{}
		for _, l := range stored {
			total += l.LineTotal
			if l.ProductID == unknown.ID && l.Quantity > 0 {
				for q := int64(0); q < l.Quantity; q++ {
					gotPrices = append(gotPrices, l.UnitPrice)
				}
			}
		}
		if total != quote.ReplacementTotalCents-quote.ReturnedTotalCents {
			t.Fatalf("trial %d: stored %d, quoted %d", trial, total, quote.ReplacementTotalCents-quote.ReturnedTotalCents)
		}
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
		sort.Slice(gotPrices, func(i, j int) bool { return gotPrices[i] < gotPrices[j] })
		if len(prices) != len(gotPrices) {
			t.Fatalf("trial %d: unknown items sold at %v, cart had %v", trial, gotPrices, prices)
		}
		for i := range prices {
			if prices[i] != gotPrices[i] {
				t.Fatalf("trial %d: unknown items sold at %v, cart had %v", trial, gotPrices, prices)
			}
		}
	}
}
