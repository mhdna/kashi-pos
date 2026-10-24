package main

import (
	"context"
	"encoding/json"
	"testing"
)

// discount_toggle: a salesperson can turn a line's 30% or its deal off, and
// the line pays the list price; the quote says what was given up. Without
// the setting the till refuses it.
func TestDiscountToggle(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	q := shop.store.Queries
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc-30", Name: "Thirty", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-JACKET", Discount: 30}},
	}); err != nil {
		t.Fatal(err)
	}
	cart := func(items ...salesInvoiceItem) string {
		raw, _ := json.Marshal(items)
		return string(raw)
	}
	threeShirtsAndJacket := func(off bool) string {
		return cart(
			salesInvoiceItem{Barcode: "bc-A", Qty: 1}, salesInvoiceItem{Barcode: "bc-B", Qty: 1},
			salesInvoiceItem{Barcode: "bc-C", Qty: 1, NoDiscount: off},
			salesInvoiceItem{Barcode: "bc-JACKET", Qty: 1, NoDiscount: off},
		)
	}

	quote, err := shop.app.QuoteInvoiceTotal(threeShirtsAndJacket(false))
	if err != nil {
		t.Fatal(err)
	}
	if quote.DiscountedSubtotalCents != 10000+8000+0+7000 {
		t.Fatalf("with discounts: %d, want the free shirt and 30%% off the jacket", quote.DiscountedSubtotalCents)
	}

	if _, err := shop.app.QuoteInvoiceTotal(threeShirtsAndJacket(true)); err == nil {
		t.Fatal("turning a discount off must be refused while discount_toggle is off")
	}

	if err := shop.app.SetDiscountToggleEnabled(true); err != nil {
		t.Fatal(err)
	}
	quote, err = shop.app.QuoteInvoiceTotal(threeShirtsAndJacket(true))
	if err != nil {
		t.Fatal(err)
	}
	if quote.DiscountedSubtotalCents != 10000+8000+5000+10000 {
		t.Fatalf("discounts off: %d, want every line at list price (two shirts make no set)", quote.DiscountedSubtotalCents)
	}
	off := map[string]LineQuote{}
	for _, line := range quote.Lines {
		if line.DiscountOff {
			off[line.Code] = line
		}
		if line.Deal != "" && line.DealDiscountCents != 0 {
			t.Fatalf("no deal should give anything now: %+v", line)
		}
	}
	if off["JACKET"].OffDiscount != 30 || off["C"].OffDealLabel != "B2G1" || len(off) != 2 {
		t.Fatalf("the quote should say what each line gave up: %+v", off)
	}

	// Checkout charges the same.
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := shop.app.salesInvoiceTx(SalesInvoiceParams{
		ItemsJSON: threeShirtsAndJacket(true), PaymentsJSON: string(payments), ClientID: shop.client.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GrandTotal != quote.GrandTotalCents {
		t.Fatalf("checkout charged %d, the quote said %d", result.GrandTotal, quote.GrandTotalCents)
	}
}
