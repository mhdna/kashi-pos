package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"testing"
)

func TestRenderDesignerPreview(t *testing.T) {
	shop := DesignerShop{StoreName: "Bazaar", Branch: "Centro", Phone: "03 111 222", ReturnPolicy: "Swaps in 7 days."}
	for _, mode := range receiptPreviewModes {
		out, err := RenderDesignerPreview(DesignerInput{Theme: "classic", Layout: "A", Mode: mode, Shop: shop})
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(out.PNG))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != out.Width || img.Bounds().Dy() != out.Height {
			t.Fatalf("%s: size %dx%d, said %dx%d", mode, img.Bounds().Dx(), img.Bounds().Dy(), out.Width, out.Height)
		}
		groups := map[string]bool{}
		for _, m := range out.Marks {
			if groups[m.Group] {
				t.Fatalf("%s: %s marked twice", mode, m.Group)
			}
			groups[m.Group] = true
			if m.Top < 0 || m.Bottom > out.Height || m.Top >= m.Bottom {
				t.Fatalf("%s: %s %d-%d", mode, m.Group, m.Top, m.Bottom)
			}
		}
		for _, g := range []string{"header", "meta", "columns", "totals", "payments", "policy", "socials", "thanks"} {
			if !groups[g] {
				t.Fatalf("%s: no %s to click", mode, g)
			}
		}
		if groups["referral"] != (mode == receiptModeReferral) {
			t.Fatalf("%s: referral part %v", mode, groups["referral"])
		}
	}
	// The design changes the drawing; the shop's own data is on it.
	var texts []string
	saved := receiptTextDrawn
	receiptTextDrawn = func(v string, _ int) { texts = append(texts, v) }
	_, err := RenderDesignerPreview(DesignerInput{Mode: "referral", Shop: shop, ReferralPercent: 20,
		Texts: `{"elements":{"thanks":{"text":"Bye","case":"upper"}},"hide":["policy"]}`})
	receiptTextDrawn = saved
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Bazaar", "Centro", "BYE", "20%"} {
		if !drewText(texts, want) {
			t.Fatalf("%q not drawn: %q", want, texts)
		}
	}
	if drewText(texts, "Swaps in 7 days.") {
		t.Fatal("left-out policy drawn")
	}
	// The referral block left out prints no code.
	texts = nil
	receiptTextDrawn = func(v string, _ int) { texts = append(texts, v) }
	_, _ = RenderDesignerPreview(DesignerInput{Mode: "referral", Shop: shop, Texts: `{"hide":["referral"]}`})
	receiptTextDrawn = saved
	if drewText(texts, sampleReferralCode) {
		t.Fatal("hidden referral printed its code")
	}
}

func TestNormalizeReceiptTextsJSON(t *testing.T) {
	for in, want := range map[string]string{
		"":                                   "",
		"{oops":                              "",
		"{}":                                 "",
		`{"elements":{"nope":{"text":"x"}}}`: "",
		`{"elements":{"thanks":{"text":" Bye "}},"date_format":"iso"}`: `{"elements":{"thanks":{"text":"Bye"}},"date_format":"iso"}`,
	} {
		if got := NormalizeReceiptTextsJSON(in); got != want {
			t.Fatalf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestDesignerCatalog(t *testing.T) {
	raw, err := json.Marshal(DesignerCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Groups   []receiptGroupInfo   `json:"groups"`
		Elements []receiptElementInfo `json:"elements"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, g := range c.Groups {
		known[g.Key] = true
	}
	seen := map[string]bool{}
	for _, e := range c.Elements {
		if !known[e.Group] {
			t.Fatalf("%s is in no part", e.Key)
		}
		if seen[e.Key] {
			t.Fatalf("%s twice", e.Key)
		}
		seen[e.Key] = true
		if e.Can == 0 {
			t.Fatalf("%s can change nothing", e.Key)
		}
	}
	for g := range known {
		found := false
		for _, e := range c.Elements {
			found = found || e.Group == g
		}
		if !found {
			t.Fatalf("part %s has nothing to change", g)
		}
	}
}

// Central draws previews for several people at once: no shared state.
func TestRenderDesignerPreviewConcurrently(t *testing.T) {
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			_, err := RenderDesignerPreview(DesignerInput{Mode: receiptPreviewModes[i%len(receiptPreviewModes)]})
			done <- err
		}(i)
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkRenderDesignerPreview(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := RenderDesignerPreview(DesignerInput{Theme: "classic", Layout: "A", Mode: receiptModeReferral}); err != nil {
			b.Fatal(err)
		}
	}
}
