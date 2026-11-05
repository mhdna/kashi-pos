package main

import (
	"context"
	"encoding/json"
	"testing"
)

// kashi's live cart overrides price a sale past the branch's settings: with
// custom prices, custom discounts and the deal toggle all off, kashi can still
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
