package main

import (
	"context"
	"encoding/json"
	"testing"
)

// discount_toggle and deal_toggle: a salesperson can turn a line's 30% off,
// or its deal, each under its own switch, and the line goes without it; the
// quote says what was given up. Without the switch the till refuses it.
func TestDiscountAndDealToggles(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	q := shop.store.Queries
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc-30", Name: "Thirty", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-JACKET", Discount: 30}},
	}); err != nil {
		t.Fatal(err)
	}
	cart := func(noDiscount, noDeal bool) string {
		raw, _ := json.Marshal([]salesInvoiceItem{
			{Barcode: "bc-A", Qty: 1}, {Barcode: "bc-B", Qty: 1},
			{Barcode: "bc-C", Qty: 1, NoDeal: noDeal},
			{Barcode: "bc-JACKET", Qty: 1, NoDiscount: noDiscount},
		})
		return string(raw)
	}
	quote := func(noDiscount, noDeal bool) InvoiceQuote {
		t.Helper()
		quote, err := shop.app.QuoteInvoiceTotal(cart(noDiscount, noDeal))
		if err != nil {
			t.Fatal(err)
		}
		return quote
	}
	const shirts, freeShirtSet, jacket, jacketAt30 = 10000 + 8000 + 5000, 10000 + 8000, 10000, 7000

	if got := quote(false, false).DiscountedSubtotalCents; got != freeShirtSet+jacketAt30 {
		t.Fatalf("with both: %d, want the free shirt and 30%% off the jacket", got)
	}
	if _, err := shop.app.QuoteInvoiceTotal(cart(true, false)); err == nil {
		t.Fatal("turning a discount off must be refused while discount_toggle is off")
	}
	if _, err := shop.app.QuoteInvoiceTotal(cart(false, true)); err == nil {
		t.Fatal("turning a deal off must be refused while deal_toggle is off")
	}

	// The discount switch turns off the 30%, not the deal.
	if err := shop.app.SetDiscountToggleEnabled(true); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.app.QuoteInvoiceTotal(cart(false, true)); err == nil {
		t.Fatal("discount_toggle must not let a deal be turned off")
	}
	got := quote(true, false)
	if got.DiscountedSubtotalCents != freeShirtSet+jacket {
		t.Fatalf("30%% off: %d, want the deal kept and the jacket at list price", got.DiscountedSubtotalCents)
	}
	for _, line := range got.Lines {
		if (line.Code == "JACKET") != (line.OffDiscount == 30) || line.OffDealLabel != "" {
			t.Fatalf("only the jacket gave up 30%%: %+v", line)
		}
	}

	// The deal switch turns off the deal: two shirts make no set.
	if err := shop.app.SetDealToggleEnabled(true); err != nil {
		t.Fatal(err)
	}
	got = quote(false, true)
	if got.DiscountedSubtotalCents != shirts+jacketAt30 {
		t.Fatalf("deal off: %d, want every shirt at list price and the jacket still 30%% off", got.DiscountedSubtotalCents)
	}
	for _, line := range got.Lines {
		if (line.Code == "C") != (line.OffDealLabel == "B2G1") || line.OffDiscount != 0 {
			t.Fatalf("only shirt C gave up its deal: %+v", line)
		}
	}

	// Checkout charges what the quote said.
	both := quote(true, true)
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: both.GrandTotalCents}})
	result, err := shop.app.salesInvoiceTx(SalesInvoiceParams{
		ItemsJSON: cart(true, true), PaymentsJSON: string(payments), ClientID: shop.client.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.GrandTotal != both.GrandTotalCents || both.DiscountedSubtotalCents != shirts+jacket {
		t.Fatalf("checkout charged %d, the quote said %d (%d before tax)", result.GrandTotal, both.GrandTotalCents, both.DiscountedSubtotalCents)
	}
}
