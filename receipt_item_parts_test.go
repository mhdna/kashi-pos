package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// The designer's item settings (Items and column titles): which parts of each
// item row print. Name, colour and size and the before-and-after line print by
// default; the code under the name only when switched on.

func partsTexts(elements map[string]receiptElement) receiptTexts {
	return receiptTexts{Elements: elements}
}

func TestItemsAsPrinted(t *testing.T) {
	skirt := invoiceItem{Name: "Lady Leather Skirt", Code: "TNV551219", Barcode: "800011", Color: "Black", Size: "M", Price: 45, Discount: 30, Qty: 1}
	scarf := invoiceItem{Name: "Wool Scarf", Barcode: "8000000000777", Price: 25, Qty: 1}
	free := invoiceItem{Name: "Sport Cap" + freeTag, Code: "CP-001", Price: 9, Qty: 1, Free: true, DealDiscount: 9, Deal: "Buy 2 get 1"}
	cases := []struct {
		name     string
		elements map[string]receiptElement
		item     invoiceItem
		wantName string
		wantCode string
		variant  bool
		note     bool
	}{
		{"default", nil, skirt, "Lady Leather Skirt", "", true, true},
		{"code on", map[string]receiptElement{"item_code": {Show: true}}, skirt, "Lady Leather Skirt", "TNV551219", true, true},
		{"code on, barcode when no code", map[string]receiptElement{"item_code": {Show: true}}, scarf, "Wool Scarf", "8000000000777", true, false},
		{"name off, code on: code in its place", map[string]receiptElement{"item_name": {Hide: true}, "item_code": {Show: true}}, skirt, "TNV551219", "", true, true},
		{"name and code off", map[string]receiptElement{"item_name": {Hide: true}}, skirt, "", "", true, true},
		{"colour and size off", map[string]receiptElement{"item_variant": {Hide: true}}, skirt, "Lady Leather Skirt", "", false, true},
		{"before and after off", map[string]receiptElement{"item_price_note": {Hide: true}}, skirt, "Lady Leather Skirt", "", true, false},
		{"free item keeps its tag", nil, free, "Sport Cap (FREE)", "", true, true},
		{"free item, name off, code on", map[string]receiptElement{"item_name": {Hide: true}, "item_code": {Show: true}}, free, "CP-001 (FREE)", "", true, true},
		{"free item, own tag words", map[string]receiptElement{"deal_free": {Text: "gift"}}, free, "Sport Cap gift", "", true, true},
		{"free item, tag off, name off", map[string]receiptElement{"deal_free": {Hide: true}, "item_name": {Hide: true}}, free, "", "", true, true},
		// Show on a part that is on by default, or Hide on the code, mean
		// nothing; normalizeReceiptElements drops them, and they don't print.
		{"hide on code is not a switch", map[string]receiptElement{"item_code": {Hide: true}}, skirt, "Lady Leather Skirt", "", true, true},
	}
	for _, c := range cases {
		got := itemsAsPrinted(partsTexts(c.elements), []invoiceItem{c.item})[0]
		if got.Name != c.wantName || itemCodeUnderName(got) != c.wantCode {
			t.Errorf("%s: name %q code %q, want %q %q", c.name, got.Name, itemCodeUnderName(got), c.wantName, c.wantCode)
		}
		if (variantText(got) != "") != (c.variant && variantText(c.item) != "") {
			t.Errorf("%s: variant %q", c.name, variantText(got))
		}
		if (priceNoteText(got) != "") != (c.note && priceNoteText(c.item) != "") {
			t.Errorf("%s: price note %q", c.name, priceNoteText(got))
		}
		// Amounts never change: a setting only hides words.
		g1, d1, n1 := itemAmounts(c.item)
		g2, d2, n2 := itemAmounts(got)
		if g1 != g2 || d1 != d2 || n1 != n2 {
			t.Errorf("%s: amounts changed", c.name)
		}
	}
}

// Over random items and random settings, each row prints exactly what the
// settings say, and the money on it is untouched.
func TestItemsAsPrintedRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	pick := func(xs ...string) string { return xs[rng.Intn(len(xs))] }
	for i := 0; i < 1000; i++ {
		item := invoiceItem{
			Name: pick("Jeans", "Summer Floral Dress", "", "Cap"), Code: pick("", "JN-220", " "), Barcode: pick("", "800123"),
			Color: pick("", "Navy", "-"), Size: pick("", "M"), Price: float64(rng.Intn(9000)) / 100, Discount: float64(rng.Intn(3) * 10), Qty: float64(1 + rng.Intn(3)),
		}
		if rng.Intn(4) == 0 {
			item.Free, item.Name, item.DealDiscount = true, item.Name+freeTag, item.Price*item.Qty
		}
		hideName, hideVariant, showCode, hideNote, hideFree := rng.Intn(2) == 0, rng.Intn(2) == 0, rng.Intn(2) == 0, rng.Intn(2) == 0, rng.Intn(3) == 0
		el := map[string]receiptElement{
			"item_name": {Hide: hideName}, "item_variant": {Hide: hideVariant}, "item_code": {Show: showCode},
			"item_price_note": {Hide: hideNote}, "deal_free": {Hide: hideFree},
		}
		got := itemsAsPrinted(partsTexts(el), []invoiceItem{item})[0]
		label := strings.TrimSuffix(item.Name, freeTag)
		code := itemCodeText(item)
		wantName, wantCode := label, ""
		if showCode {
			wantCode = code
		}
		if hideName {
			wantName, wantCode = wantCode, ""
		}
		if item.Free && !hideFree {
			wantName = strings.TrimSpace(wantName + freeTag)
		}
		ctx := fmt.Sprintf("case %d %+v name=%v variant=%v code=%v note=%v free=%v", i, item, !hideName, !hideVariant, showCode, !hideNote, !hideFree)
		if got.Name != wantName || itemCodeUnderName(got) != wantCode {
			t.Fatalf("%s: name %q code %q, want %q %q", ctx, got.Name, itemCodeUnderName(got), wantName, wantCode)
		}
		if hideVariant && variantText(got) != "" || !hideVariant && variantText(got) != variantText(item) {
			t.Fatalf("%s: variant %q", ctx, variantText(got))
		}
		if hideNote && priceNoteText(got) != "" || !hideNote && priceNoteText(got) != priceNoteText(item) {
			t.Fatalf("%s: note %q", ctx, priceNoteText(got))
		}
		g1, d1, n1 := itemAmounts(item)
		g2, d2, n2 := itemAmounts(got)
		if g1 != g2 || d1 != d2 || n1 != n2 || got.Qty != item.Qty || got.Price != item.Price {
			t.Fatalf("%s: money changed", ctx)
		}
	}
}

func TestItemPartSettingsAreKeptOnlyAsTheyMeanSomething(t *testing.T) {
	out := normalizeReceiptElements(map[string]receiptElement{
		"item_code":       {Show: true, Hide: true, Text: "x"},
		"item_name":       {Hide: true, Show: true, Size: 14},
		"item_variant":    {Show: true},
		"item_price_note": {Hide: true},
	}, strings.TrimSpace)
	if out["item_code"] != (receiptElement{Show: true}) {
		t.Errorf("item_code = %+v", out["item_code"])
	}
	if out["item_name"] != (receiptElement{Hide: true}) {
		t.Errorf("item_name = %+v", out["item_name"])
	}
	if _, ok := out["item_variant"]; ok {
		t.Errorf("item_variant kept a Show it can't use: %+v", out["item_variant"])
	}
	if out["item_price_note"] != (receiptElement{Hide: true}) {
		t.Errorf("item_price_note = %+v", out["item_price_note"])
	}
	for _, key := range []string{"item_name", "item_variant", "item_code", "item_price_note"} {
		info, ok := receiptElementByKey(key)
		if !ok || info.Group != "columns" {
			t.Errorf("%s is not in Items and column titles: %+v", key, info)
		}
	}
}

// drawnFor renders every look with the settings and returns what each drew.
func drawnFor(t *testing.T, elements map[string]receiptElement, fn func(look string, drawn []string)) {
	t.Helper()
	for _, width := range []int64{80, 58} {
		for _, theme := range receiptThemeCatalog() {
			for _, layout := range theme.Layouts {
				var drawn []string
				receiptTextDrawn = func(value string, _ int) { drawn = append(drawn, strings.TrimSpace(value)) }
				data := sampleReceiptData(printReceiptData{ReceiptWidth: width, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
				data.Items = itemCodeTestItems()
				data.Texts = partsTexts(elements)
				_, err := renderReceiptPreview(data, theme.ID, layout.ID)
				receiptTextDrawn = nil
				look := fmt.Sprintf("%dmm %s/%s", width, theme.ID, layout.ID)
				if err != nil {
					t.Fatalf("%s: %v", look, err)
				}
				fn(look, drawn)
			}
		}
	}
}

func drew(drawn []string, pred func(string) bool) bool {
	for _, d := range drawn {
		if pred(d) {
			return true
		}
	}
	return false
}

func TestEveryLookPrintsTheCodeWhenSwitchedOn(t *testing.T) {
	drawnFor(t, map[string]receiptElement{"item_code": {Show: true}}, func(look string, drawn []string) {
		for _, code := range []string{"TNV551219-16", "TNV551219", "8000000000777"} {
			if !drew(drawn, func(d string) bool { return codeOf(d) == code }) {
				t.Errorf("%s: code %s not printed with Code on", look, code)
			}
		}
	})
}

func TestEveryLookLeavesNamesOutAndPrintsCodesInstead(t *testing.T) {
	drawnFor(t, map[string]receiptElement{"item_name": {Hide: true}, "item_code": {Show: true}}, func(look string, drawn []string) {
		for _, name := range []string{"Lady Leather Skirt", "Wool Scarf", "Gift Wrap"} {
			if drew(drawn, func(d string) bool { return d != "" && isNameLine(d, name) }) {
				t.Errorf("%s: name %q printed with Name off", look, name)
			}
		}
		// On narrow paper a long code wraps in the name column like a name.
		// The row's quantity and amount are drawn between the halves.
		var label []string
		for _, d := range drawn {
			if !strings.HasPrefix(d, "$") && strings.Trim(d, "0123456789") != "" {
				label = append(label, d)
			}
		}
		if !strings.Contains(strings.Join(label, ""), "TNV551219-16") {
			t.Errorf("%s: the code didn't take the name's place", look)
		}
	})
}

func TestEveryLookLeavesOutTheBeforeAndAfterLine(t *testing.T) {
	was := func(d string) bool { return strings.HasPrefix(strings.ToUpper(d), "WAS $") }
	drawnFor(t, nil, func(look string, drawn []string) {
		if !drew(drawn, was) {
			t.Errorf("%s: no before-and-after line by default for a 30%% off item", look)
		}
	})
	drawnFor(t, map[string]receiptElement{"item_price_note": {Hide: true}}, func(look string, drawn []string) {
		if drew(drawn, was) {
			t.Errorf("%s: before-and-after line printed with it off", look)
		}
	})
}

func TestEveryLookLeavesOutColourAndSize(t *testing.T) {
	black := func(d string) bool { return strings.Contains(strings.ToUpper(d), "BLACK") }
	printsVariant := map[string]bool{}
	drawnFor(t, nil, func(look string, drawn []string) { printsVariant[look] = drew(drawn, black) })
	if len(printsVariant) == 0 || !func() bool {
		for _, v := range printsVariant {
			if v {
				return true
			}
		}
		return false
	}() {
		t.Fatal("no look prints colour and size; the test checks nothing")
	}
	drawnFor(t, map[string]receiptElement{"item_variant": {Hide: true}}, func(look string, drawn []string) {
		if drew(drawn, black) {
			t.Errorf("%s: colour and size printed with it off", look)
		}
	})
}

func TestEveryLookPrintsWithEveryItemPartOff(t *testing.T) {
	drawnFor(t, map[string]receiptElement{
		"item_name": {Hide: true}, "item_variant": {Hide: true}, "item_price_note": {Hide: true},
	}, func(look string, drawn []string) {
		for _, d := range drawn {
			if strings.Contains(d, "Leather") || strings.Contains(strings.ToUpper(d), "WAS $") {
				t.Errorf("%s: printed %q with every part off", look, d)
			}
		}
	})
}

func TestESCPosReceiptFollowsItemParts(t *testing.T) {
	text := func(el map[string]receiptElement) string {
		return string(buildESCPosReceipt(printReceiptData{
			StoreName: "Centro", ReceiptNo: "SA/CT/2026/03191", Items: itemCodeTestItems(), Texts: partsTexts(el),
			GrandTotalCents: 9900, RoundingCurrency: testLBPReceiptCurrency, ReceiptWidth: 80,
		}))
	}
	def := text(nil)
	if strings.Contains(def, "TNV551219") || !strings.Contains(def, "Lady Leather Skirt") || !strings.Contains(def, "Was $") {
		t.Fatalf("default text receipt: %q", def)
	}
	on := text(map[string]receiptElement{"item_code": {Show: true}})
	if !strings.Contains(on, "  TNV551219-16") || !strings.Contains(on, "  8000000000777") {
		t.Fatalf("code on: %q", on)
	}
	swapped := text(map[string]receiptElement{"item_name": {Hide: true}, "item_code": {Show: true}, "item_price_note": {Hide: true}})
	if strings.Contains(swapped, "Leather") || strings.Contains(swapped, "Was $") || !strings.Contains(swapped, "TNV551219-16") {
		t.Fatalf("name off, code on, note off: %q", swapped)
	}
}
