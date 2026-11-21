package main

import "testing"

// A student deal that sits on the items (no invoice discount) still prints
// the "any deal" banner; with no deal at all it does not.
func TestAnyDealBannerCoversDealsOnItems(t *testing.T) {
	b := receiptBanner{Style: "ribbon", Title: "STUDENT DEAL", When: "deal"}
	cases := []struct {
		name string
		data printReceiptData
		want bool
	}{
		{"nothing", printReceiptData{Items: []invoiceItem{{}}}, false},
		{"item deal", printReceiptData{Items: []invoiceItem{{}, {Deal: "Students 15%"}}}, true},
		{"invoice deal", printReceiptData{InvoiceDiscount: invoiceDiscount{Name: "Students 15%", Cents: 500}}, true},
	}
	for _, c := range cases {
		cv := &receiptCanvas{data: c.data}
		if got := cv.bannerApplies(b); got != c.want {
			t.Errorf("%s: applies=%v want %v", c.name, got, c.want)
		}
	}
}

func TestBannerStyleAndOwnStyleSurviveNormalizing(t *testing.T) {
	in := receiptTexts{BannerStyle: "burst", Banners: []receiptBanner{{Style: "ribbon", Title: "A", OwnStyle: true}, {Style: "burst", Title: "B"}}}
	out := normalizeReceiptTexts(in)
	if out.BannerStyle != "burst" || !out.Banners[0].OwnStyle || out.Banners[1].OwnStyle {
		t.Fatalf("lost: %+v", out)
	}
	if normalizeReceiptTexts(receiptTexts{BannerStyle: "nope"}).BannerStyle != "" {
		t.Fatal("unknown global style kept")
	}
}
