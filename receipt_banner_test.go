package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func bannerTexts(b ...receiptBanner) receiptTexts {
	return normalizeReceiptTexts(receiptTexts{Banners: b})
}

// The reported case: ready-made banners like BUY 1 GET 1 print where and
// when they are set to.
func TestReceiptBannersPrint(t *testing.T) {
	render := func(mode string, texts receiptTexts, deal *DesignerDeal) ([]string, []DesignerMark, int) {
		var drawn []string
		saved := receiptTextDrawn
		receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
		defer func() { receiptTextDrawn = saved }()
		raw, _ := json.Marshal(texts)
		out, err := RenderDesignerPreview(DesignerInput{Mode: mode, Texts: string(raw), Deal: deal})
		if err != nil {
			t.Fatal(err)
		}
		return drawn, out.Marks, out.Height
	}
	b1g1 := receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", Subtitle: "FREE", When: "item_deal"}
	student := receiptBanner{Style: "stamp", Title: "STUDENT DEAL", When: "deal:Students 15%"}
	always := receiptBanner{Style: "ornament", Title: "Thank You", Place: "bottom"}
	texts := bannerTexts(b1g1, student, always)

	// The reported case: one banner per invoice. A buy 2 get 1 sale has the
	// buy X get Y banner and the always-on one; only the first prints.
	drawn, marks, h := render(receiptModeItemDeal, texts, nil)
	if !drewText(drawn, "BUY 1 GET 1") || drewText(drawn, "Thank You") || drewText(drawn, "STUDENT DEAL") {
		t.Fatalf("buy 2 get 1 sale: %q", drawn)
	}
	// Under the shop's name, before the customer.
	at := func(s string) int {
		for i, d := range drawn {
			if d == s {
				return i
			}
		}
		return -1
	}
	if !(at("BUY 1 GET 1") > at("Sales Invoice: PV-10452") && at("BUY 1 GET 1") < at("Customer: Maya Haddad")) {
		t.Fatalf("top banner out of place: %q", drawn)
	}
	seen := map[string]bool{}
	for _, m := range marks {
		seen[m.Group] = true
		if m.Top < 0 || m.Bottom > h {
			t.Fatalf("%s outside the paper", m.Group)
		}
	}
	if !seen["banner-0"] || seen["banner-2"] || seen["banner-1"] {
		t.Fatalf("banner marks: %v", seen)
	}
	// A plain sale: the always-on one, at the end.
	drawn, _, _ = render(receiptModeSale, texts, nil)
	if !drewText(drawn, "Thank You") {
		t.Fatalf("plain sale: %q", drawn)
	}
	thanks, line := -1, -1
	for i, d := range drawn {
		if d == "Thank You" {
			thanks = i
		}
		if d == "Thank you for shopping with us!" {
			line = i
		}
	}
	if thanks < line {
		t.Fatalf("bottom banner not at the end: %q", drawn)
	}

	drawn, _, _ = render(receiptModeClient, texts, &DesignerDeal{Name: "Students 15%", Kind: "percent", Percent: 15})
	if !drewText(drawn, "STUDENT DEAL") || drewText(drawn, "BUY 1 GET 1") {
		t.Fatalf("student sale: %q", drawn)
	}
	drawn, _, _ = render(receiptModeClient, texts, &DesignerDeal{Name: "Other", Kind: "percent", Percent: 5})
	if drewText(drawn, "STUDENT DEAL") {
		t.Fatal("a banner for one deal printed with another")
	}
	drawn, _, _ = render(receiptModeSale, bannerTexts(receiptBanner{Style: "frame", Title: "SALE", When: "deal"}), nil)
	if drewText(drawn, "SALE") {
		t.Fatal("a deal banner printed without a deal")
	}

	// Every style draws, and a banner makes the receipt taller.
	_, _, plain := render(receiptModeSale, receiptTexts{}, nil)
	for _, st := range receiptBannerStyles {
		drawn, _, h := render(receiptModeSale, bannerTexts(receiptBanner{Style: st.Key, Title: "HELLO", Subtitle: "there"}), nil)
		if !drewText(drawn, "HELLO") || !drewText(drawn, "there") || h <= plain {
			t.Fatalf("%s: %q, %d vs %d", st.Key, drawn, h, plain)
		}
	}
}

func TestNormalizeReceiptBanners(t *testing.T) {
	long := strings.Repeat("W", 100)
	out := normalizeReceiptBanners([]receiptBanner{
		{Style: "ribbon", Title: "  BUY 1 GET 1 ", When: "item_deal", Place: "bottom"},
		{Style: "nope", Title: "X"},
		{Style: "frame", Title: "  "},
		{Style: "stars", Title: long, Subtitle: long, Place: "side"},
		// A condition this till doesn't know: left out, never printed always.
		{Style: "stars", Title: "FUTURE", When: "weird"},
		{Style: "stamp", Title: "A", When: "deal: "},
		{Style: "stamp", Title: "B", When: "deal:Students 15%"},
		{Style: "frame", Title: "C"},
	})
	// Kept: the first, the long always one, Students and C; the bad style,
	// the empty title, a deal with no name and the unknown condition go.
	if len(out) != 4 {
		t.Fatalf("%d banners kept", len(out))
	}
	for _, b := range out {
		if b.Title == "FUTURE" {
			t.Fatal("a banner with an unknown condition was kept")
		}
	}
	// No more than receiptBannerMax are kept.
	many := make([]receiptBanner, receiptBannerMax+3)
	for i := range many {
		many[i] = receiptBanner{Style: "frame", Title: fmt.Sprint(i)}
	}
	if got := normalizeReceiptBanners(many); len(got) != receiptBannerMax || got[receiptBannerMax-1].Title != fmt.Sprint(receiptBannerMax-1) {
		t.Fatalf("kept %d", len(got))
	}
	if out[0] != (receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", When: "item_deal", Place: "bottom"}) {
		t.Fatalf("first: %+v", out[0])
	}
	if len([]rune(out[1].Title)) != receiptBannerTitleMax || len([]rune(out[1].Subtitle)) != receiptBannerSubtitleMax || out[1].When != "" || out[1].Place != "" {
		t.Fatalf("second: %+v", out[1])
	}
	if out[2].When != "deal:Students 15%" || out[3].Title != "C" {
		t.Fatalf("kept: %+v", out[2:])
	}
	// Presets are all valid, and draw on their own.
	if n := min(len(receiptBannerPresets), receiptBannerMax); len(normalizeReceiptBanners(receiptBannerPresets[:n])) != n {
		t.Fatal("a preset doesn't survive cleaning")
	}
	for _, p := range receiptBannerPresets {
		raw, _ := json.Marshal(p)
		png, err := RenderBannerPreview(string(raw))
		if err != nil || len(png) < 100 {
			t.Fatalf("%s: %v", p.Title, err)
		}
	}
	if _, err := RenderBannerPreview(`{"style":"nope","title":"x"}`); err == nil {
		t.Fatal("an unknown style drew")
	}
}

// Deals that added up print one banner: the biggest deal's.
func TestOneBannerForStackedDeals(t *testing.T) {
	texts := bannerTexts(
		receiptBanner{Style: "ribbon", Title: "TEN OFF", When: "deal:10% above $50"},
		receiptBanner{Style: "stamp", Title: "STUDENT DEAL", When: "deal:Students 15%"},
		receiptBanner{Style: "burst", Title: "BIG SPENDER", When: "deal:20% above $150"},
		receiptBanner{Style: "frame", Title: "ANY DEAL", When: "deal"},
		receiptBanner{Style: "stars", Title: "HELLO"},
	)
	data := receiptPreviewData(designBase(), receiptModeSale)
	data.Texts = texts
	draw := func(name string) []string {
		data.InvoiceDiscount = invoiceDiscount{Name: name, Cents: 500}
		if name == "" {
			data.InvoiceDiscount.Cents = 0
		}
		drawn, _, _ := drawnTexts(t, data, "classic", "A")
		var titles []string
		for _, d := range drawn {
			for _, b := range texts.Banners {
				if d == b.Title {
					titles = append(titles, d)
				}
			}
		}
		return titles
	}
	cases := map[string]string{
		"20% above $150 + Students 15% + 10% above $50": "BIG SPENDER",
		"Students 15% + 10% above $50":                  "STUDENT DEAL",
		"10% above $50":                                 "TEN OFF",
		"Staff $5":                                      "ANY DEAL",
		"":                                              "HELLO",
	}
	for name, want := range cases {
		if got := draw(name); len(got) != 1 || got[0] != want {
			t.Fatalf("%q: printed %q, want only %q", name, got, want)
		}
	}
}

// An item on sale (its own discount) shows the item-discount banner; a plain
// sale and a deal's lines don't.
func TestItemDiscountBanner(t *testing.T) {
	texts := bannerTexts(receiptBanner{Style: "ribbon", Title: "SALE", When: "item_discount"})
	draw := func(items []invoiceItem) bool {
		data := receiptPreviewData(designBase(), receiptModeSale)
		data.Texts = texts
		data.Items = items
		drawn, _, _ := drawnTexts(t, data, "classic", "A")
		return drewText(drawn, "SALE")
	}
	if !draw([]invoiceItem{{Name: "Shirt", Price: 20, Qty: 1, Discount: 10}}) {
		t.Fatal("an item on sale printed no banner")
	}
	if draw([]invoiceItem{{Name: "Shirt", Price: 20, Qty: 1}}) {
		t.Fatal("a plain sale printed the sale banner")
	}
	if draw([]invoiceItem{{Name: "Shirt", Price: 20, Qty: 1, Discount: 100, Deal: "Buy 2 get 1", Free: true}}) {
		t.Fatal("a deal's free line printed the sale banner")
	}
}

// A banner can name its deal: {deal} prints the deal's name, the title in
// capitals; the referral banner prints for a customer with a code to share.
func TestBannerDealNameAndReferral(t *testing.T) {
	texts := bannerTexts(
		receiptBanner{Style: "ribbon", Title: "{deal}", Subtitle: "{deal} today", When: "item_deal"},
		receiptBanner{Style: "stamp", Title: "SHARE YOUR CODE", When: "referral"},
	)
	draw := func(mode string) []string {
		data := receiptPreviewData(designBase(), mode)
		data.Texts = texts
		drawn, _, _ := drawnTexts(t, data, "classic", "A")
		return drawn
	}
	// The reported case: the buy 2 get 1 view's banner says BUY 2 GET 1.
	if d := draw(receiptModeItemDeal); !drewText(d, "BUY 2 GET 1") || !drewText(d, "Buy 2 get 1 today") || drewText(d, "SHARE YOUR CODE") {
		t.Fatalf("item deal: %q", d)
	}
	if d := draw(receiptModeReferral); !drewText(d, "SHARE YOUR CODE") {
		t.Fatalf("referral: %q", d)
	}
	if d := draw(receiptModeSale); drewText(d, "SHARE YOUR CODE") || drewText(d, "BUY 2 GET 1") {
		t.Fatalf("plain sale: %q", d)
	}
}

// The reported case: the generic deal banner says BUY 1 GET 1 or BUY 2 GET 1
// after the deal on the invoice, however long the deal's own name.
func TestBannerDealTitleBuyGet(t *testing.T) {
	cases := map[string]string{
		"Buy 2 get 1 free":                 "BUY 2 GET 1",
		"Buy 2 get 1 free (otherwise 30%)": "BUY 2 GET 1",
		"Buy 1 Get 1":                      "BUY 1 GET 1",
		"buy3get2":                         "BUY 3 GET 2",
		"Students 15%":                     "STUDENTS 15%",
		"":                                 "",
	}
	for name, want := range cases {
		if got := bannerDealTitle(name); got != want {
			t.Fatalf("%q: %q, want %q", name, got, want)
		}
	}
	texts := bannerTexts(receiptBanner{Style: "ribbon", Title: "{deal}", When: "item_deal"})
	for _, deal := range []string{"Buy 1 Get 1", "Buy 2 get 1 free (otherwise 30%)"} {
		data := receiptPreviewData(designBase(), receiptModeItemDeal)
		data.Texts = texts
		for i := range data.Items {
			if data.Items[i].Deal != "" {
				data.Items[i].Deal = deal
			}
		}
		drawn, _, _ := drawnTexts(t, data, "classic", "A")
		if want := bannerDealTitle(deal); !drewText(drawn, want) || drewText(drawn, "OTHERWISE") {
			t.Fatalf("%s: %q", deal, drawn)
		}
	}
}
