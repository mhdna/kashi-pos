package main

import (
	"encoding/json"
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

	drawn, marks, h := render(receiptModeItemDeal, texts, nil)
	if !drewText(drawn, "BUY 1 GET 1") || !drewText(drawn, "Thank You") || drewText(drawn, "STUDENT DEAL") {
		t.Fatalf("buy 2 get 1 sale: %q", drawn)
	}
	// Under the shop's name, before the customer; the bottom one at the end.
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
	if at("Thank You") < at("Thank you for shopping with us!") {
		t.Fatalf("bottom banner not at the end: %q", drawn)
	}
	seen := map[string]bool{}
	for _, m := range marks {
		seen[m.Group] = true
		if m.Top < 0 || m.Bottom > h {
			t.Fatalf("%s outside the paper", m.Group)
		}
	}
	if !seen["banner-0"] || !seen["banner-2"] || seen["banner-1"] {
		t.Fatalf("banner marks: %v", seen)
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
		{Style: "stars", Title: long, Subtitle: long, When: "weird", Place: "side"},
		{Style: "stamp", Title: "A", When: "deal: "},
		{Style: "stamp", Title: "B", When: "deal:Students 15%"},
		{Style: "frame", Title: "C"},
	})
	if len(out) != receiptBannerMax {
		t.Fatalf("%d banners kept", len(out))
	}
	if out[0] != (receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", When: "item_deal", Place: "bottom"}) {
		t.Fatalf("first: %+v", out[0])
	}
	if len([]rune(out[1].Title)) != receiptBannerTitleMax || len([]rune(out[1].Subtitle)) != receiptBannerSubtitleMax || out[1].When != "" || out[1].Place != "" {
		t.Fatalf("second: %+v", out[1])
	}
	if out[2].When != "" || out[3].When != "deal:Students 15%" {
		t.Fatalf("whens: %q %q", out[2].When, out[3].When)
	}
	// Presets are all valid, and draw on their own.
	if len(normalizeReceiptBanners(receiptBannerPresets[:receiptBannerMax])) != receiptBannerMax {
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
