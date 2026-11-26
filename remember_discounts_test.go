package main

import (
	"encoding/json"
	"math/rand"
	"sort"
	"testing"
)

// Central turns remembering on and off by command, the till reports what it
// runs with, and a till never told keeps it off.
func TestRememberItemDiscountsFollowsCentral(t *testing.T) {
	app, _, _, _, _ := newExchangeReplacementPricingFixture(t)
	check := func(want bool) {
		t.Helper()
		settings, err := app.GetAppSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings["rememberItemDiscounts"] != want || app.flagEnabled(app.ctx, rememberItemDiscountsFlag) != want {
			t.Fatalf("remember = %v, want %v", settings["rememberItemDiscounts"], want)
		}
	}
	check(false)
	for _, tc := range []struct {
		payload string
		want    bool
	}{
		{`{"remember_item_discounts":true}`, true},
		{`{}`, true}, // a push that doesn't name it leaves it alone
		{`{"remember_item_discounts":false}`, false},
	} {
		var p updateSettingsCommandPayload
		if err := json.Unmarshal([]byte(tc.payload), &p); err != nil {
			t.Fatal(err)
		}
		if p.RememberItemDiscounts != nil {
			if err := app.SetRememberItemDiscounts(*p.RememberItemDiscounts); err != nil {
				t.Fatal(err)
			}
		}
		check(tc.want)
	}
}

// A typed discount stays on its own pieces, so the screen builds lines whose
// units have different discounts. Each cart is quoted and checked out, and
// what is stored is what each piece was given.
func TestSaleAcceptsLinesWithDifferentDiscountsPerPiece(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 150; trial++ {
		app, _, _, product, cashbox := newExchangeReplacementPricingFixture(t)
		if err := app.SetCustomItemDiscountsEnabled(true); err != nil {
			t.Fatal(err)
		}
		item := salesInvoiceItem{Barcode: product.Barcode, Code: product.Code}
		var want []int64 // each piece's discount
		var total int64
		for n := 1 + rng.Intn(5); n > 0; n-- {
			discount := rng.Int63n(60)
			if len(want) > 0 && rng.Intn(3) == 0 {
				discount = want[rng.Intn(len(want))] // pieces alike
			}
			item.Qty++
			item.UnitDiscounts = append(item.UnitDiscounts, discount)
			want = append(want, discount)
			total += 500 - 5*discount
		}
		raw := mustJSON(t, []salesInvoiceItem{item})
		quote, err := app.QuoteInvoiceTotal(raw, 0)
		if err != nil {
			t.Fatalf("trial %d: refused at quote: %v\n%s", trial, err, raw)
		}
		if quote.DiscountedSubtotalCents != total {
			t.Fatalf("trial %d: quoted %d, want %d\n%s", trial, quote.DiscountedSubtotalCents, total, raw)
		}
		payments := mustJSON(t, []checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
		result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: raw, PaymentsJSON: payments})
		if err != nil {
			t.Fatalf("trial %d: refused at checkout: %v\n%s", trial, err, raw)
		}
		stored, err := app.store.ListInvoiceProducts(app.ctx, 2) // 1 is the fixture's sale
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		var sum int64
		for _, l := range stored {
			sum += l.LineTotal
			for i := int64(0); i < l.Quantity; i++ {
				got = append(got, l.Discount)
			}
		}
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		if sum != total || result.GrandTotal != quote.GrandTotalCents || mustJSON(t, got) != mustJSON(t, want) {
			t.Fatalf("trial %d: stored %v for %d, want %v for %d", trial, got, sum, want, total)
		}
	}
}
