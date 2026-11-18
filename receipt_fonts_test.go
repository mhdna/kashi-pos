package main

import (
	"bytes"
	"encoding/json"
	"image"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// fontMarks draws data with the given elements and groups and gives each
// line's font by its key (the first time it's drawn).
func fontMarks(t *testing.T, mode, theme, layout string, elements, groups map[string]receiptElement) (map[string]string, image.Image) {
	t.Helper()
	base := designBase()
	base.Website = "bazaar.example"
	data := receiptPreviewData(base, strings.TrimSuffix(strings.TrimSuffix(mode, "+deal"), "+round"))
	if strings.HasSuffix(mode, "+round") {
		data.GrandTotalCents += 7
	}
	if strings.HasSuffix(mode, "+deal") {
		applyDesignerDeal(&data, DesignerDeal{Name: "Students 15%", Kind: "percent", Percent: 15})
	}
	data.Texts = normalizeReceiptTexts(receiptTexts{Elements: elements, Groups: groups})
	data.ReferralNote = data.Texts.referralNote(sampleReferralPercent)
	_, img, marks := drawnTexts(t, data, theme, layout)
	fonts := map[string]string{}
	for _, m := range marks {
		if m.Kind != "line" {
			continue
		}
		if _, ok := fonts[m.Part]; !ok {
			fonts[m.Part] = m.Font
		}
	}
	return fonts, img
}

func sameImage(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			r1, _, _, _ := a.At(x, y).RGBA()
			r2, _, _, _ := b.At(x, y).RGBA()
			if r1 != r2 {
				return false
			}
		}
	}
	return true
}

// The reported case: every line that can take a font prints in it, and only
// that line changes font.
func TestReceiptLineFonts(t *testing.T) {
	covered := map[string]bool{}
	modes := append([]string{receiptModeItemDeal, receiptModeClient + "+deal", receiptModeSale + "+round"}, receiptPreviewModes...)
	type look struct{ id, layout string }
	var looks []look
	for _, th := range receiptThemeCatalog() {
		for _, l := range th.Layouts {
			looks = append(looks, look{th.ID, l.ID})
		}
	}
	for _, theme := range looks {
		// Each line once per look, where it first shows.
		done := map[string]bool{}
		for _, mode := range modes {
			base, baseImg := fontMarks(t, mode, theme.id, theme.layout, nil, nil)
			for key, f := range base {
				if f == "" {
					t.Fatalf("%s/%s: %s drawn in a font off the list", theme.id, mode, key)
				}
			}
			for _, info := range receiptElements {
				if info.Can&canFont == 0 || base[info.Key] == "" || done[info.Key] {
					continue
				}
				done[info.Key] = true
				want := "pacifico"
				if base[info.Key] == want {
					want = "oswald"
				}
				got, img := fontMarks(t, mode, theme.id, theme.layout, map[string]receiptElement{info.Key: {Font: want}}, nil)
				if got[info.Key] != want {
					t.Fatalf("%s/%s: %s drawn in %q, want %q", theme.id, mode, info.Key, got[info.Key], want)
				}
				for key, f := range base {
					if key != info.Key && got[key] != f {
						t.Fatalf("%s/%s: setting %s's font changed %s from %q to %q", theme.id, mode, info.Key, key, f, got[key])
					}
				}
				if sameImage(img, baseImg) {
					t.Fatalf("%s/%s: %s in %s prints the same", theme.id, mode, info.Key, want)
				}
				covered[info.Key] = true
			}
		}
	}
	for _, info := range receiptElements {
		if info.Can&canFont != 0 && !covered[info.Key] {
			t.Errorf("%s can take a font but was never seen drawn", info.Key)
		}
	}
}

// A part's font sets all its lines; a line's own wins.
func TestReceiptPartFont(t *testing.T) {
	got, _ := fontMarks(t, receiptModeSale, "classic", "A",
		map[string]receiptElement{"subtotal": {Font: "lobster"}},
		map[string]receiptElement{"totals": {Font: "bebas-neue"}})
	if got["subtotal"] != "lobster" {
		t.Fatalf("subtotal in %q", got["subtotal"])
	}
	n := 0
	for _, info := range receiptElements {
		if info.Group == "totals" && info.Key != "subtotal" && info.Can&canFont != 0 && got[info.Key] != "" {
			n++
			if got[info.Key] != "bebas-neue" {
				t.Fatalf("%s in %q, want the part's bebas-neue", info.Key, got[info.Key])
			}
		}
	}
	if n == 0 {
		t.Fatal("no totals line seen")
	}
	if got["store_name"] == "bebas-neue" {
		t.Fatal("the totals' font leaked into the header")
	}
}

func TestReceiptFontsClean(t *testing.T) {
	texts := normalizeReceiptTexts(receiptTexts{
		Elements: map[string]receiptElement{"store_name": {Font: "nope"}, "subtotal": {Font: " inter "}, "invoice_label_sale": {Font: "inter"}},
		Groups:   map[string]receiptElement{"header": {Font: "comic"}},
		Banners:  []receiptBanner{{Style: "ribbon", Title: "X", Font: "lobster"}, {Style: "ribbon", Title: "Y", Font: "x"}},
	})
	if _, ok := texts.Elements["store_name"]; ok {
		t.Fatalf("unknown font kept: %+v", texts.Elements)
	}
	if texts.Elements["subtotal"].Font != "inter" {
		t.Fatalf("subtotal: %+v", texts.Elements["subtotal"])
	}
	// The invoice title can't change size, so it can't change font either.
	if texts.Elements["invoice_label_sale"].Font != "" || texts.Groups != nil {
		t.Fatalf("kept: %+v %+v", texts.Elements["invoice_label_sale"], texts.Groups)
	}
	if texts.Banners[0].Font != "lobster" || texts.Banners[1].Font != "" {
		t.Fatalf("banners: %+v", texts.Banners)
	}
}

// Every font on the list parses, draws every letter a receipt uses, and
// its file is served for the designer.
func TestReceiptFontList(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range receiptFonts {
		if seen[f.Key] || f.Label == "" || f.Scale <= 0 {
			t.Fatalf("bad entry %+v", f.Key)
		}
		seen[f.Key] = true
		for _, bold := range []bool{false, true} {
			data, ok := ReceiptFontFile(f.Key, bold)
			if !ok {
				t.Fatal(f.Key)
			}
			parsed, err := opentype.Parse(data)
			if err != nil {
				t.Fatalf("%s: %v", f.Key, err)
			}
			face, _ := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 24, DPI: 72})
			for r := rune(0x21); r < 0x7f; r++ {
				if adv, ok := face.GlyphAdvance(r); !ok || adv <= 0 {
					t.Fatalf("%s lacks %q", f.Key, r)
				}
			}
			if font.MeasureString(face, "Total $12.50") <= 0 {
				t.Fatal(f.Key)
			}
		}
	}
	if _, ok := ReceiptFontFile("nope", false); ok {
		t.Fatal("an unknown font served")
	}
	cat, _ := json.Marshal(DesignerCatalog())
	if !bytes.Contains(cat, []byte(`"key":"pacifico"`)) || !bytes.Contains(cat, []byte(`"font":64`)) {
		t.Fatal("the catalog lacks the fonts")
	}
}

// A banner's title takes any font.
func TestReceiptBannerFont(t *testing.T) {
	plain, _ := json.Marshal(receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1"})
	fancy, _ := json.Marshal(receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", Font: "lobster"})
	a, err := RenderBannerPreview(string(plain))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := RenderBannerPreview(string(fancy))
	if bytes.Equal(a, b) {
		t.Fatal("the banner's font changed nothing")
	}
}

// The other currency's total line took no size or weight from the designer
// on the printed picture; now it does, like the rows above it.
func TestReceiptEquivalentStyled(t *testing.T) {
	for _, look := range []struct{ id, layout string }{{"classic", "A"}, {"modern", "A"}, {"boutique", "A"}} {
		_, plain := fontMarks(t, receiptModeSale, look.id, look.layout, nil, nil)
		b := false
		_, big := fontMarks(t, receiptModeSale, look.id, look.layout, map[string]receiptElement{"equivalent": {Size: 4, Bold: &b}}, nil)
		if sameImage(plain, big) {
			t.Fatalf("%s: the equivalent line ignored its size", look.id)
		}
	}
}
