package main

import (
	"reflect"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestDealUnitDiscounts(t *testing.T) {
	b2g1 := dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100}
	b1g1half := dealTerms{Kind: dealBuyGet, BuyQty: 1, GetQty: 1, Percent: 50}
	threeFor50 := dealTerms{Kind: dealBundlePrice, BuyQty: 3, BundlePrice: 5000}
	volume := dealTerms{Kind: dealVolumePercent, BuyQty: 3, Percent: 20}
	b2g1else30 := dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: 30}

	for _, tc := range []struct {
		name   string
		terms  dealTerms
		prices []int64
		want   []int64
	}{
		{"b2g1: the cheapest is free", b2g1, []int64{10000, 8000, 5000}, []int64{0, 0, 5000}},
		{"b2g1: two short of a set gives nothing", b2g1, []int64{10000, 8000}, []int64{0, 0}},
		{"b2g1: six units, the two cheapest overall", b2g1, []int64{100, 600, 200, 500, 300, 400}, []int64{100, 0, 200, 0, 0, 0}},
		{"b2g1: seven units still two", b2g1, []int64{1, 2, 3, 4, 5, 6, 7}, []int64{1, 2, 0, 0, 0, 0, 0}},
		{"equal prices: the first in order", b2g1, []int64{900, 900, 900}, []int64{900, 0, 0}},
		{"2nd 50% off", b1g1half, []int64{2000, 1001}, []int64{0, 501}},
		{"3 for $50 shares the saving by price", threeFor50, []int64{2000, 2000, 3000}, []int64{571, 571, 858}},
		{"3 for $50 not taken when dearer", threeFor50, []int64{1000, 1000, 1000}, []int64{0, 0, 0}},
		{"3 for $50: the fourth unit is extra", threeFor50, []int64{2000, 2000, 2000, 9000}, []int64{333, 333, 334, 0}},
		{"fallback: one unit is 30% off", b2g1else30, []int64{10000}, []int64{3000}},
		{"fallback: two units are both 30% off", b2g1else30, []int64{10000, 8000}, []int64{3000, 2400}},
		{"fallback: a full set is the deal alone", b2g1else30, []int64{10000, 8000, 5000}, []int64{0, 0, 5000}},
		{"fallback: the dearest unit beside a set gets it", b2g1else30, []int64{5000, 10000, 8000, 6000}, []int64{5000, 3000, 0, 0}},
		{"fallback: two sets, nothing left over", b2g1else30, []int64{1, 2, 3, 4, 5, 6}, []int64{1, 2, 0, 0, 0, 0}},
		{"no fallback: units outside a set stay full price", b2g1, []int64{10000, 8000, 5000, 6000}, []int64{0, 0, 5000, 0}},
		{"3 or more 20% off", volume, []int64{1000, 2000, 3000}, []int64{200, 400, 600}},
		{"2 is not 3 or more", volume, []int64{1000, 2000}, []int64{0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.terms.unitDiscounts(tc.prices, roundingHalfUp)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unitDiscounts(%v) = %v, want %v", tc.prices, got, tc.want)
			}
		})
	}
}

// A line whose units come out differently is split, the free unit on a line
// of its own; lines not on a deal stay where they were.
func TestApplyDealsSplitsLines(t *testing.T) {
	deals := map[string]dealTerms{"b2g1": {UID: "b2g1", Name: "Buy 2 get 1", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100}}
	line := func(code string, price, qty int64, deal string) pricedInvoiceProduct {
		return pricedInvoiceProduct{product: db.Product{Code: code}, unitPrice: price, qty: qty, lineTotal: price * qty, dealUID: deal}
	}
	out := applyDeals([]pricedInvoiceProduct{
		line("SHIRT", 1000, 2, "b2g1"),
		line("BAG", 3000, 1, ""),
		line("SOCK", 500, 1, "b2g1"),
		line("CAP", 700, 2, "b2g1"),
	}, deals, roundingHalfUp)

	type row struct {
		code             string
		qty, deal, total int64
		free             bool
	}
	var got []row
	for _, p := range out {
		got = append(got, row{p.product.Code, p.qty, p.dealDiscount, p.lineTotal, p.isFree()})
	}
	// Five deal units: one set, so the cheapest (the sock) is free.
	want := []row{
		{"SHIRT", 2, 0, 2000, false},
		{"BAG", 1, 0, 3000, false},
		{"SOCK", 1, 500, 0, true},
		{"CAP", 2, 0, 1400, false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %+v\nwant %+v", got, want)
	}

	// Six units: the two cheapest, a sock and a cap, so the caps split.
	out = applyDeals([]pricedInvoiceProduct{
		line("SHIRT", 1000, 3, "b2g1"), line("SOCK", 500, 1, "b2g1"), line("CAP", 700, 2, "b2g1"),
	}, deals, roundingHalfUp)
	got = nil
	for _, p := range out {
		got = append(got, row{p.product.Code, p.qty, p.dealDiscount, p.lineTotal, p.isFree()})
	}
	want = []row{
		{"SHIRT", 3, 0, 3000, false},
		{"SOCK", 1, 500, 0, true},
		{"CAP", 1, 0, 700, false},
		{"CAP", 1, 700, 0, true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %+v\nwant %+v", got, want)
	}
}

// A deal the till has no terms for gives nothing and the line forgets it.
func TestApplyDealsUnknownDeal(t *testing.T) {
	out := applyDeals([]pricedInvoiceProduct{
		{unitPrice: 100, qty: 3, lineTotal: 300, dealUID: "gone"},
	}, map[string]dealTerms{}, roundingHalfUp)
	if len(out) != 1 || out[0].dealUID != "" || out[0].lineTotal != 300 {
		t.Fatalf("got %+v", out)
	}
}

func TestDealTermsLabel(t *testing.T) {
	cases := []struct {
		terms dealTerms
		want  string
	}{
		{dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100}, "B2G1"},
		{dealTerms{Kind: dealBuyGet, BuyQty: 1, GetQty: 1, Percent: 50}, "B1G1 50%"},
		{dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: 30}, "B2G1 else 30%"},
		{dealTerms{Kind: dealBundlePrice, BuyQty: 3, BundlePrice: 5000}, "3 for $50"},
		{dealTerms{Kind: dealBundlePrice, BuyQty: 2, BundlePrice: 2550}, "2 for $25.50"},
		{dealTerms{Kind: dealVolumePercent, BuyQty: 3, Percent: 20}, "3+ 20%"},
		{dealTerms{Kind: "other", Name: "Spring"}, "Spring"},
	}
	for _, c := range cases {
		if got := c.terms.label(); got != c.want {
			t.Errorf("%+v: label() = %q, want %q", c.terms, got, c.want)
		}
	}
}

func TestDealTermsIsRewardUnit(t *testing.T) {
	b2g1else30 := dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: 30}
	b2g1 := dealTerms{Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100}
	bundle := dealTerms{Kind: dealBundlePrice, BuyQty: 3, BundlePrice: 5000}
	for _, tc := range []struct {
		name  string
		terms dealTerms
		price int64
		deal  int64
		want  bool
	}{
		{"a free unit", b2g1else30, 5000, 5000, true},
		{"a unit at the fallback is no reward", b2g1else30, 10000, 3000, false},
		{"a paid unit", b2g1else30, 10000, 0, false},
		{"without a fallback any saving is the reward", b2g1, 10000, 3000, true},
		{"a bundle has no reward units", bundle, 2000, 333, false},
	} {
		if got := tc.terms.isRewardUnit(tc.price, tc.deal, roundingHalfUp); got != tc.want {
			t.Errorf("%s: isRewardUnit = %v, want %v", tc.name, got, tc.want)
		}
	}
}
