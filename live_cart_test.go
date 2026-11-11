package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
)

// central's live cart overrides price a sale past the branch's settings: with
// custom prices, custom discounts and the deal toggle all off, central can still
// turn every deal off and give a line a price or a discount. They belong to
// the sale they were made for and go when the screen moves on.
func TestLiveCartOverrides(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	app := shop.app
	if err := applySyncedDiscountList(ctx, shop.store.Queries, syncedDiscountList{
		UID: "disc-30", Name: "Thirty", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-JACKET", Discount: 30}},
	}); err != nil {
		t.Fatal(err)
	}
	cart, _ := json.Marshal([]salesInvoiceItem{
		{Barcode: "bc-A", Qty: 1}, {Barcode: "bc-B", Qty: 1}, {Barcode: "bc-C", Qty: 1},
		{Barcode: "bc-JACKET", Qty: 1},
	})
	subtotal := func() int64 {
		t.Helper()
		quote, err := app.QuoteInvoiceTotal(string(cart))
		if err != nil {
			t.Fatal(err)
		}
		return quote.DiscountedSubtotalCents
	}
	report := func(saleRef, mode string) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"saleRef": saleRef, "mode": mode, "lines": []any{}})
		if err := app.ReportLiveCart(string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	override := func(o liveCartOverrides) error {
		raw, _ := json.Marshal(o)
		return app.executeCartOverrideCommand(raw)
	}
	price := func(cents int64) *int64 { return &cents }

	const shirts, freeShirtSet, jacketAt30 = 10000 + 8000 + 5000, 10000 + 8000, 7000
	if got := subtotal(); got != freeShirtSet+jacketAt30 {
		t.Fatalf("before: %d, want the free shirt and 30%% off the jacket", got)
	}

	report("sale-1", "sale")
	if err := override(liveCartOverrides{SaleRef: "sale-0", AllDealsOff: true}); err == nil {
		t.Fatal("an override for a sale no longer on screen must be refused")
	}

	// Every deal off: the deal toggle is off, and it still goes.
	if err := override(liveCartOverrides{SaleRef: "sale-1", AllDealsOff: true}); err != nil {
		t.Fatal(err)
	}
	if got := subtotal(); got != shirts+jacketAt30 {
		t.Fatalf("deals off: %d, want every shirt at list price", got)
	}

	// A price and a discount, with custom prices and discounts off.
	if err := override(liveCartOverrides{SaleRef: "sale-1", Lines: []liveCartLineOverride{
		{Barcode: "bc-JACKET", PriceCents: price(9000), Discount: price(50)},
	}}); err != nil {
		t.Fatal(err)
	}
	if got := subtotal(); got != freeShirtSet+4500 {
		t.Fatalf("jacket at $90 less 50%%: %d, want %d (deals back on)", got, freeShirtSet+4500)
	}

	// Checkout charges it.
	quote, err := app.QuoteInvoiceTotal(string(cart))
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := app.salesInvoiceTx(SalesInvoiceParams{
		ItemsJSON: string(cart), PaymentsJSON: string(payments), ClientID: shop.client.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GrandTotal != quote.GrandTotalCents {
		t.Fatalf("charged %d, quoted %d", result.GrandTotal, quote.GrandTotalCents)
	}

	// The next sale doesn't inherit them, and an exchange takes none.
	report("sale-2", "sale")
	if got := subtotal(); got != freeShirtSet+jacketAt30 {
		t.Fatalf("next sale: %d, want list pricing back", got)
	}
	report("sale-3", "exchange")
	if err := override(liveCartOverrides{SaleRef: "sale-3", AllDealsOff: true}); err == nil {
		t.Fatal("an exchange must refuse overrides")
	}
}

// liveCartShop is the deal shop with an unknown item, on sale "sale-1".
func liveCartShop(t *testing.T) dealShop {
	t.Helper()
	shop := setUpDealShop(t)
	if _, err := shop.app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"saleRef": "sale-1", "mode": "sale", "lines": []any{}})
	if err := shop.app.ReportLiveCart(string(raw)); err != nil {
		t.Fatal(err)
	}
	return shop
}

func setOverrides(t *testing.T, app *App, o liveCartOverrides) {
	t.Helper()
	o.SaleRef = "sale-1"
	raw, _ := json.Marshal(o)
	if err := app.executeCartOverrideCommand(raw); err != nil {
		t.Fatal(err)
	}
}

func cents(v int64) *int64 { return &v }

// Central sets an unknown item's price, discount and deal from the live cart,
// one line at a time though every unknown item shares a barcode: each line
// goes by its ref, and the others stay as the cashier rang them.
func TestLiveCartUnknownItemOverrides(t *testing.T) {
	shop := liveCartShop(t)
	app := shop.app
	A, B := salesInvoiceItem{Barcode: "bc-A", Qty: 1}, salesInvoiceItem{Barcode: "bc-B", Qty: 1}
	cart := []salesInvoiceItem{A, B, unknownLine("u1", 3000, ""), unknownLine("u2", 4000, unknownDeal20)}
	total := func(items ...salesInvoiceItem) int64 { t.Helper(); return quoteItems(t, app, items...).GrandTotalCents }
	before := total(cart...)

	// u2's price: the cart prices as if the cashier had typed $50 on u2.
	want := total(A, B, unknownLine("u1", 3000, ""), unknownLine("u2", 5000, unknownDeal20))
	setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{
		{Barcode: "UNKNOWN", LineRef: "u2", PriceCents: cents(5000)},
	}})
	if got := total(cart...); got != want || got == before {
		t.Fatalf("u2 at $50: %d, want %d (before %d)", got, want, before)
	}

	// u1 out of the deal, by its choice or by "no deal": like picking "none".
	setOverrides(t, app, liveCartOverrides{})
	want = total(A, B, unknownLine("u1", 3000, unknownDealNone), unknownLine("u2", 4000, unknownDeal20))
	for _, l := range []liveCartLineOverride{
		{Barcode: "UNKNOWN", LineRef: "u1", UnknownDeal: unknownDealNone},
		{Barcode: "UNKNOWN", LineRef: "u1", NoDeal: true},
	} {
		setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{l}})
		if got := total(cart...); got != want {
			t.Fatalf("u1 off the deal (%+v): %d, want %d", l, got, want)
		}
	}

	// A discount, with the branch's custom discounts off: half off, and like
	// any saving it takes the line off the deal.
	if app.flagEnabled(app.ctx, customItemDiscountsFlag) {
		t.Fatal("the test needs custom discounts off")
	}
	setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{
		{Barcode: "UNKNOWN", LineRef: "u1", Discount: cents(50)},
	}})
	if got := total(unknownLine("u1", 3000, "")); got != 1500 {
		t.Fatalf("u1 at half off: %d, want 1500", got)
	}
	if lines := unknownQuoteLines(quoteItems(t, app, unknownLine("u1", 3000, ""))); len(lines) != 1 || lines[0].UnknownDeal != unknownDealNone {
		t.Fatalf("u1 with a discount stays off the deal: %+v", lines)
	}

	// Checkout charges what the quote said.
	setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{
		{Barcode: "UNKNOWN", LineRef: "u2", PriceCents: cents(5000), UnknownDeal: unknownDeal30},
		{Barcode: "UNKNOWN", LineRef: "u1", Discount: cents(10)},
	}})
	quote := quoteItems(t, app, cart...)
	itemsJSON, _ := json.Marshal(cart)
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	app.unknownSessionRef = ""
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: shop.client.ID})
	if err != nil {
		t.Fatal(err)
	}
	if sale.GrandTotal != quote.GrandTotalCents {
		t.Fatalf("charged %d, quoted %d", sale.GrandTotal, quote.GrandTotalCents)
	}
}

// An unknown line's override never lands on another line: not on a product
// with the same barcode and no ref, not on an unknown line by barcode alone.
func TestLiveCartUnknownOverrideStaysOnItsLine(t *testing.T) {
	app := liveCartShop(t).app
	cart := []salesInvoiceItem{{Barcode: "bc-A", Qty: 1}, unknownLine("u1", 3000, unknownDealNone)}
	before := quoteItems(t, app, cart...).GrandTotalCents
	setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{
		{Barcode: "UNKNOWN", PriceCents: cents(100)},             // no ref: no unknown line
		{Barcode: "bc-A", LineRef: "u1", PriceCents: cents(100)}, // a ref: not bc-A
		{Barcode: "UNKNOWN", LineRef: "u9", PriceCents: cents(100)},
	}})
	if got := quoteItems(t, app, cart...).GrandTotalCents; got != before {
		t.Fatalf("total %d, want %d unchanged", got, before)
	}
}

// What central may send for an unknown line is checked on the till too.
func TestLiveCartUnknownOverrideRefusals(t *testing.T) {
	app := liveCartShop(t).app
	for name, l := range map[string]liveCartLineOverride{
		"deal choice off an unknown line": {Barcode: "bc-A", UnknownDeal: unknownDeal30},
		"made-up deal choice":             {Barcode: "UNKNOWN", LineRef: "u1", UnknownDeal: "b2g1_99"},
		"price 0":                         {Barcode: "UNKNOWN", LineRef: "u1", PriceCents: cents(0)},
		"discount 101":                    {Barcode: "UNKNOWN", LineRef: "u1", Discount: cents(101)},
	} {
		raw, _ := json.Marshal(liveCartOverrides{SaleRef: "sale-1", Lines: []liveCartLineOverride{l}})
		if err := app.executeCartOverrideCommand(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Over hundreds of random carts and overrides, a cart central changed prices
// exactly as the same cart typed that way at the till, line by line, and its
// lines add up to its total.
func TestLiveCartUnknownOverridesRandomized(t *testing.T) {
	app := liveCartShop(t).app
	// The typed reference needs custom discounts; central's never does.
	if err := app.setFeatureFlagBool(app.ctx, customItemDiscountsFlag, true); err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	choices := []string{"", unknownDeal30, unknownDeal20, unknownDealNone}
	for iter := 0; iter < 300; iter++ {
		var cart, typed []salesInvoiceItem
		var lines []liveCartLineOverride
		for _, bc := range []string{"bc-A", "bc-B", "bc-C"} {
			if rng.Intn(2) == 0 {
				cart = append(cart, salesInvoiceItem{Barcode: bc, Qty: 1})
				typed = append(typed, salesInvoiceItem{Barcode: bc, Qty: 1})
			}
		}
		for u := 0; u < 1+rng.Intn(4); u++ {
			ref := fmt.Sprintf("u%d", u)
			price := int64(500 + rng.Intn(200)*50)
			line := unknownLine(ref, price, choices[rng.Intn(len(choices))])
			cart = append(cart, line)
			ref2 := line
			o := liveCartLineOverride{Barcode: "UNKNOWN", LineRef: ref}
			if rng.Intn(2) == 0 {
				p := int64(100 + rng.Intn(300)*50)
				o.PriceCents, ref2.UnitPricesCents = &p, []int64{p}
			}
			if rng.Intn(3) == 0 {
				d := int64(rng.Intn(101))
				o.Discount, ref2.UnitDiscounts = &d, []int64{d}
			}
			switch rng.Intn(3) {
			case 0:
				o.UnknownDeal = choices[1+rng.Intn(3)]
				ref2.UnknownDeal = o.UnknownDeal
			case 1:
				if rng.Intn(2) == 0 {
					o.NoDeal, ref2.UnknownDeal = true, unknownDealNone
				}
			}
			typed = append(typed, ref2)
			if o.sets() {
				lines = append(lines, o)
			}
		}
		setOverrides(t, app, liveCartOverrides{})
		want := quoteItems(t, app, typed...)
		setOverrides(t, app, liveCartOverrides{Lines: lines})
		got := quoteItems(t, app, cart...)
		if got.GrandTotalCents != want.GrandTotalCents {
			t.Fatalf("iter %d: central's cart %d, typed cart %d\noverrides %+v", iter, got.GrandTotalCents, want.GrandTotalCents, lines)
		}
		var sum int64
		for _, l := range got.Lines {
			sum += l.LineTotalCents
		}
		if sum != got.DiscountedSubtotalCents {
			t.Fatalf("iter %d: lines add up to %d, subtotal %d", iter, sum, got.DiscountedSubtotalCents)
		}
	}
}
