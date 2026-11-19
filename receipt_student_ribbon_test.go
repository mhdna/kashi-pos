package main

import (
	"fmt"
	"strings"
	"testing"
)

// The reported case: the Student Deal banner is a ribbon like Buy 1 Get 1,
// the same height, even though its longer title has to shrink to fit.
func TestRibbonsAreOneHeightWhateverTheTitle(t *testing.T) {
	for _, width := range []int64{80, 58} {
		for _, sub := range []string{"", "FREE"} {
			heights := map[string]int{}
			for _, title := range []string{"BUY 1 GET 1", "STUDENT DEAL", "SALE", "STUDENT DEAL FOR EVERY UNIVERSITY CARD"} {
				data := sampleReceiptData(printReceiptData{ReceiptWidth: width, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
				data.Texts = bannerTexts(receiptBanner{Style: "ribbon", Title: title, Subtitle: sub})
				var drawn []string
				receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
				_, marks, err := renderReceiptPreviewMarked(data, defaultReceiptTheme, defaultReceiptLayout)
				receiptTextDrawn = nil
				if err != nil {
					t.Fatal(err)
				}
				h := -1
				for _, m := range marks {
					if m.Part == "banner-0" {
						h = m.Bottom - m.Top
					}
				}
				if h <= 0 {
					t.Fatalf("%dmm %q: no banner drawn", width, title)
				}
				if !drewText(drawn, title) {
					t.Fatalf("%dmm %q: title not drawn: %q", width, title, drawn)
				}
				heights[title] = h
			}
			want := heights["BUY 1 GET 1"]
			for title, h := range heights {
				if h != want {
					t.Errorf("%dmm subtitle %q: %q ribbon is %d px tall, BUY 1 GET 1 is %d", width, sub, title, h, want)
				}
			}
		}
	}
}

func TestStudentDealPresetIsARibbon(t *testing.T) {
	var student, b1g1 *receiptBanner
	for i := range receiptBannerPresets {
		switch receiptBannerPresets[i].Title {
		case "STUDENT DEAL":
			student = &receiptBannerPresets[i]
		case "BUY 1 GET 1":
			b1g1 = &receiptBannerPresets[i]
		}
	}
	if student == nil || b1g1 == nil {
		t.Fatal("presets missing")
	}
	if student.Style != b1g1.Style || student.Style != "ribbon" || student.When != "deal" {
		t.Fatalf("student preset %+v, buy 1 get 1 %+v", *student, *b1g1)
	}
}

// Only the first name: "Rana Haddad" was "Rana H.".
func TestReceiptSalesmanIsTheFirstName(t *testing.T) {
	for in, want := range map[string]string{
		"Rana Haddad":         "Rana",
		"Mohammad Ali Hassan": "Mohammad",
		"  Rana   Haddad  ":   "Rana",
		"Rana":                "Rana",
		"رنا حداد":            "رنا",
		"":                    "Not specified",
		"   ":                 "Not specified",
	} {
		if got := formatReceiptSalesperson(in); got != want {
			t.Errorf("formatReceiptSalesperson(%q) = %q, want %q", in, got, want)
		}
	}
	data := sampleReceiptData(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
	data.Salesperson = "Rana Haddad"
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			var drawn []string
			receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
			_, err := renderReceiptPreview(data, theme.ID, layout.ID)
			receiptTextDrawn = nil
			if err != nil {
				t.Fatal(err)
			}
			look := fmt.Sprintf("%s/%s", theme.ID, layout.ID)
			if drewText(drawn, "Haddad") || drewText(drawn, "H.") {
				t.Errorf("%s: printed more than the first name: %q", look, drawn)
			}
			if !drewText(drawn, "Rana") && !drewText(drawn, "RANA") {
				t.Errorf("%s: salesman not printed", look)
			}
		}
	}
	text := string(buildESCPosReceipt(printReceiptData{StoreName: "Centro", ReceiptNo: "SA/CT/1", Salesperson: "Rana Haddad", ReceiptWidth: 80, RoundingCurrency: testLBPReceiptCurrency}))
	if strings.Contains(text, "Haddad") || !strings.Contains(text, "Rana") {
		t.Fatalf("text receipt salesman: %q", text)
	}
}
