package main

import (
	"encoding/json"
	"image"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"
)

func withTexts(data printReceiptData, t receiptTexts) printReceiptData {
	data.Texts = t
	return data
}

func inkCount(img image.Image) int {
	_, _, n := bandInk(img, 0, img.Bounds().Dy())
	return n
}

// The reported case: a whole part centred and bold at once, a line's own
// setting still winning.
func TestReceiptGroupStyle(t *testing.T) {
	yes, no := true, false
	base := receiptPreviewData(designBase(), receiptModeSale)
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			_, plain, plainMarks := drawnTexts(t, base, theme.ID, layout.ID)
			_, bold, boldMarks := drawnTexts(t, withTexts(base, receiptTexts{Groups: map[string]receiptElement{"payments": {Bold: &no}}}), theme.ID, layout.ID)
			pb, bb := groupBand(t, plainMarks, "payments"), groupBand(t, boldMarks, "payments")
			_, _, plainInk := bandInk(plain, pb.Top, pb.Bottom)
			_, _, regularInk := bandInk(bold, bb.Top, bb.Bottom)
			if regularInk >= plainInk {
				t.Fatalf("%s: the payment part made regular keeps its ink (%d vs %d)", where, regularInk, plainInk)
			}
			// The line's own bold wins over the part's.
			_, mixed, mixedMarks := drawnTexts(t, withTexts(base, receiptTexts{
				Groups:   map[string]receiptElement{"payments": {Bold: &no}},
				Elements: map[string]receiptElement{"received": {Bold: &yes}},
			}), theme.ID, layout.ID)
			mb := groupBand(t, mixedMarks, "payments")
			_, _, mixedInk := bandInk(mixed, mb.Top, mb.Bottom)
			if !(mixedInk > regularInk && mixedInk < plainInk) {
				t.Fatalf("%s: a line's own bold didn't win (%d between %d and %d)", where, mixedInk, regularInk, plainInk)
			}
			_, big, _ := drawnTexts(t, withTexts(base, receiptTexts{Groups: map[string]receiptElement{"totals": {Size: 4}}}), theme.ID, layout.ID)
			if big.Bounds().Dy() <= plain.Bounds().Dy() {
				t.Fatalf("%s: bigger totals aren't taller", where)
			}
			_, right, rightMarks := drawnTexts(t, withTexts(base, receiptTexts{Groups: map[string]receiptElement{"policy": {Align: "right"}}}), theme.ID, layout.ID)
			rb, _ := markOf(rightMarks, "line", "policy_text")
			pb2, _ := markOf(plainMarks, "line", "policy_text")
			rl, _, _ := bandInk(right, rb.Top, rb.Bottom)
			pl, _, _ := bandInk(plain, pb2.Top, pb2.Bottom)
			if rl <= pl {
				t.Fatalf("%s: the policy part set right didn't move right (%d vs %d)", where, rl, pl)
			}
		}
	}
}

// Lines and boxes change the receipt; nothing set leaves it as it was.
func TestReceiptDecor(t *testing.T) {
	base := receiptPreviewData(designBase(), receiptModeSale)
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			_, plain, _ := drawnTexts(t, base, theme.ID, layout.ID)
			for _, style := range receiptLineStyles {
				_, above, _ := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"thanks": {Above: style}}}), theme.ID, layout.ID)
				if above.Bounds().Dy() <= plain.Bounds().Dy() || inkCount(above) <= inkCount(plain) {
					t.Fatalf("%s: a %s line above the thanks drew nothing", where, style)
				}
				_, boxed, marks := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"payments": {Box: style, Thickness: 3}}}), theme.ID, layout.ID)
				band := groupBand(t, marks, "payments")
				// The box's sides run down the paper's edges inside the band.
				l, r, _ := bandInk(boxed, band.Top, band.Bottom)
				if l > 14 || r < boxed.Bounds().Dx()-15 {
					t.Fatalf("%s: %s box not at the edges: %d-%d", where, style, l, r)
				}
			}
			_, rounded, rmarks := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"policy": {Box: "solid", Rounded: true}}}), theme.ID, layout.ID)
			_, square, smarks := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"policy": {Box: "solid"}}}), theme.ID, layout.ID)
			rb, sb := groupBand(t, rmarks, "policy"), groupBand(t, smarks, "policy")
			corner := func(img image.Image, top int) bool {
				r, _, _, _ := img.At(12, top+1).RGBA()
				return r < 0x8000
			}
			if corner(rounded, rb.Top) || !corner(square, sb.Top) {
				t.Fatalf("%s: rounded corner ink %v, square %v", where, corner(rounded, rb.Top), corner(square, sb.Top))
			}
			// "none" takes the look's divider under the payments away.
			_, none, _ := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"payments": {Below: "none"}}}), theme.ID, layout.ID)
			if inkCount(none) >= inkCount(plain) || none.Bounds().Dy() > plain.Bounds().Dy() {
				t.Fatalf("%s: no divider under payments kept the ink", where)
			}
			_, thick, _ := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"payments": {Below: "solid", Thickness: 4}}}), theme.ID, layout.ID)
			_, thin, _ := drawnTexts(t, withTexts(base, receiptTexts{Decor: map[string]receiptDecor{"payments": {Below: "solid", Thickness: 1}}}), theme.ID, layout.ID)
			if inkCount(thick) <= inkCount(thin) {
				t.Fatalf("%s: a thicker line isn't heavier", where)
			}
		}
	}
}

// The parts under the totals print in the branch's order, and leaving one
// out still cuts exactly its band.
func TestReceiptFooterOrder(t *testing.T) {
	base := receiptPreviewData(designBase(), receiptModeReferral)
	order := []string{receiptSectionThanks, receiptSectionSocials, receiptSectionPayments}
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			_, img, marks := drawnTexts(t, withTexts(base, receiptTexts{Order: order}), theme.ID, layout.ID)
			top := func(part string) int { m, _ := markOf(marks, "section", part); return m.Top }
			got := []string{}
			for _, s := range []string{receiptSectionPayments, receiptSectionPolicy, receiptSectionReferral, receiptSectionSocials, receiptSectionThanks} {
				got = append(got, s)
			}
			slices.SortFunc(got, func(a, b string) int { return top(a) - top(b) })
			want := []string{receiptSectionThanks, receiptSectionSocials, receiptSectionPayments, receiptSectionPolicy, receiptSectionReferral}
			if !slices.Equal(got, want) {
				t.Fatalf("%s: parts print %v, want %v", where, got, want)
			}
			// Leaving the moved socials out cuts its band, the rest unchanged.
			full := withTexts(base, receiptTexts{Order: order})
			hidden := withTexts(base, receiptTexts{Order: order, Hide: []string{receiptSectionSocials}})
			_, hiddenImg, _ := drawnTexts(t, hidden, theme.ID, layout.ID)
			band, _ := markOf(marks, "section", receiptSectionSocials)
			if hiddenImg.Bounds().Dy() != img.Bounds().Dy()-(band.Bottom-band.Top) {
				t.Fatalf("%s: leaving the moved socials out cut %d rows, want %d", where, img.Bounds().Dy()-hiddenImg.Bounds().Dy(), band.Bottom-band.Top)
			}
			if !sameRows(img, hiddenImg, 0, 0, band.Top) || !sameRows(img, hiddenImg, band.Bottom, band.Top, img.Bounds().Dy()-band.Bottom) {
				t.Fatalf("%s: leaving the moved socials out changed the rest", where)
			}
			_ = full
		}
	}
}

// Every line whose words can change is listed, where it is, with its words.
func TestDesignerLines(t *testing.T) {
	out, err := RenderDesignerPreview(DesignerInput{Mode: receiptModeReferral, Shop: DesignerShop{StoreName: "Bazaar"}, Texts: `{"elements":{"received":{"text":"Paid"}}}`})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]DesignerLine{}
	last := 0
	for _, l := range out.Lines {
		if l.Top < last-1 || l.Bottom > out.Height || l.Top >= l.Bottom || l.SizePx <= 0 {
			t.Fatalf("%s: %d-%d after %d (height %d, size %.1f)", l.Key, l.Top, l.Bottom, last, out.Height, l.SizePx)
		}
		last = l.Top
		if _, seen := byKey[l.Key]; !seen {
			byKey[l.Key] = l
		}
	}
	for _, key := range []string{"store_name", "meta_customer", "subtotal", "payments_title", "received", "change", "policy_text", "referral_title", "thanks"} {
		if _, ok := byKey[key]; !ok {
			t.Fatalf("%s not listed", key)
		}
	}
	if byKey["referral_title"].Text != defaultReferralTitle {
		t.Fatalf("the referral title's words carry the code: %q", byKey["referral_title"].Text)
	}
	if byKey["received"].Text != "Paid" || byKey["store_name"].Text != "Bazaar" || byKey["received"].Align != "left" || byKey["thanks"].Align != "center" {
		t.Fatalf("lines: %+v %+v %+v", byKey["received"], byKey["store_name"], byKey["thanks"])
	}
}

func TestNormalizeReceiptDecorGroupsOrder(t *testing.T) {
	yes := true
	out := normalizeReceiptTexts(receiptTexts{
		Groups: map[string]receiptElement{"payments": {Bold: &yes, Size: 30, Align: "right", Case: "upper", Text: "dropped"}, "nope": {Size: 2}},
		Decor: map[string]receiptDecor{
			"policy":   {Above: "dashed", Below: "none", Box: "double", Rounded: true, Thickness: 9},
			"thanks":   {Above: "wavy", Rounded: true},
			"nonsense": {Box: "solid"},
		},
		Order: []string{"thanks", "thanks", "header", "policy"},
	})
	if g := out.Groups["payments"]; g.Size != receiptElementMaxSize || g.Bold == nil || g.Align != "right" || g.Case != "upper" || g.Text != "" {
		t.Fatalf("group %+v", g)
	}
	if _, ok := out.Groups["nope"]; ok {
		t.Fatal("unknown group kept")
	}
	if d := out.Decor["policy"]; d != (receiptDecor{Above: "dashed", Below: "none", Box: "double", Rounded: true, Thickness: 4}) {
		t.Fatalf("decor %+v", d)
	}
	if _, ok := out.Decor["thanks"]; ok {
		t.Fatal("a wavy line and corners without a box kept")
	}
	if !slices.Equal(out.Order, []string{"thanks", "policy", "payments", "referral", "socials"}) {
		t.Fatalf("order %v", out.Order)
	}
	if normalizeReceiptOrder([]string{"payments", "policy", "referral", "socials", "thanks"}) != nil {
		t.Fatal("the default order isn't stored")
	}
	raw, _ := json.Marshal(out)
	var back receiptTexts
	_ = json.Unmarshal(raw, &back)
	if again, _ := json.Marshal(normalizeReceiptTexts(back)); string(again) != string(raw) {
		t.Fatalf("not stable:\n%s\n%s", raw, again)
	}
}

// Randomized: designs with part styles, lines, boxes and orders draw on
// every look and mode, every part inside the paper.
func TestReceiptDecorRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	styles := append([]string{"", "none", "wavy"}, receiptLineStyles...)
	themes := receiptThemeCatalog()
	for run := 0; run < 120; run++ {
		texts := receiptTexts{Groups: map[string]receiptElement{}, Decor: map[string]receiptDecor{}}
		for _, g := range receiptGroups {
			if rng.Intn(2) == 0 {
				b := rng.Intn(2) == 0
				texts.Groups[g.Key] = receiptElement{Bold: &b, Size: float64(rng.Intn(9) - 3), Align: []string{"", "left", "center", "right"}[rng.Intn(4)]}
			}
			if rng.Intn(2) == 0 {
				texts.Decor[g.Key] = receiptDecor{Above: styles[rng.Intn(len(styles))], Below: styles[rng.Intn(len(styles))], Box: styles[rng.Intn(len(styles))], Rounded: rng.Intn(2) == 0, Thickness: rng.Intn(6)}
			}
		}
		order := slices.Clone(receiptFooterOrder)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		texts.Order = order[:rng.Intn(len(order)+1)]
		texts = normalizeReceiptTexts(texts)
		theme := themes[rng.Intn(len(themes))]
		layout := theme.Layouts[rng.Intn(len(theme.Layouts))]
		mode := receiptPreviewModes[rng.Intn(len(receiptPreviewModes))]
		data := receiptPreviewData(designBase(), mode)
		data.Texts = texts
		_, img, marks := drawnTexts(t, data, theme.ID, layout.ID)
		for _, m := range marks {
			if m.Top < 0 || m.Bottom > img.Bounds().Dy() || m.Top > m.Bottom {
				raw, _ := json.Marshal(texts)
				t.Fatalf("%s/%s/%s: %s %d-%d outside 0-%d (%s)", theme.ID, layout.ID, mode, m.Part, m.Top, m.Bottom, img.Bounds().Dy(), raw)
			}
		}
	}
}

// The reported case: each column title is its own words on the paper, where
// it sits on its row, never taken for the totals' TOTAL.
func TestDesignerColumnTitles(t *testing.T) {
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			var drawn []string
			saved := receiptTextDrawn
			receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
			out, err := RenderDesignerPreview(DesignerInput{Theme: theme.ID, Layout: layout.ID})
			receiptTextDrawn = saved
			if err != nil {
				t.Fatal(err)
			}
			printsItem := false
			for _, d := range drawn {
				printsItem = printsItem || d == "ITEM" || strings.HasPrefix(d, "ITEM ")
			}
			var columns DesignerMark
			for _, m := range out.Marks {
				if m.Group == "columns" {
					columns = m
				}
			}
			cols := map[string]DesignerLine{}
			for _, l := range out.Lines {
				if l.Group != "columns" {
					continue
				}
				if _, seen := cols[l.Key]; seen {
					t.Fatalf("%s: %s marked twice", where, l.Key)
				}
				cols[l.Key] = l
				if l.Left <= 0 || l.Right <= l.Left || l.Right > out.Width {
					t.Fatalf("%s: %s at x %d-%d", where, l.Key, l.Left, l.Right)
				}
				if l.Top < columns.Top || l.Bottom > columns.Bottom {
					t.Fatalf("%s: %s at %d-%d outside the items %d-%d", where, l.Key, l.Top, l.Bottom, columns.Top, columns.Bottom)
				}
			}
			if _, ok := cols["col_item"]; printsItem && !ok {
				t.Fatalf("%s: no ITEM title to click: %+v", where, cols)
			}
			// Titles on one row don't overlap.
			for a, la := range cols {
				for b, lb := range cols {
					if a < b && la.Top == lb.Top && la.Left < lb.Right && lb.Left < la.Right {
						t.Fatalf("%s: %s and %s overlap", where, a, b)
					}
				}
			}
		}
	}
	// A row's label is clickable on its own, from the paper's left.
	out, _ := RenderDesignerPreview(DesignerInput{})
	for _, l := range out.Lines {
		if l.Key == "subtotal" && (l.Left != 24 || l.Right <= l.Left || l.Right > 300) {
			t.Fatalf("subtotal label at %d-%d", l.Left, l.Right)
		}
	}
}

// The reported case: a sale with a deal shows its line, so the designer can
// see and change how each deal reads.
func TestDesignerDealPreview(t *testing.T) {
	var drawn []string
	saved := receiptTextDrawn
	receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
	defer func() { receiptTextDrawn = saved }()

	plain, err := RenderDesignerPreview(DesignerInput{Mode: receiptModeClient})
	if err != nil {
		t.Fatal(err)
	}
	drawn = nil
	students, err := RenderDesignerPreview(DesignerInput{Mode: receiptModeClient, Deal: &DesignerDeal{Name: "Students 15%", Kind: "percent", Percent: 15}})
	if err != nil {
		t.Fatal(err)
	}
	// The sample is $240.80; 15% is $36.12 off, $204.68 to pay.
	for _, want := range []string{"Students 15%", "-$36.12", "$204.68"} {
		if !drewText(drawn, want) {
			t.Fatalf("%q not drawn: %q", want, drawn)
		}
	}
	if students.Height <= plain.Height {
		t.Fatal("the deal's line takes no room")
	}
	found := false
	for _, l := range students.Lines {
		found = found || (l.Key == "invoice_discount" && l.Text == "Students 15%" && l.Left > 0)
	}
	if !found {
		t.Fatalf("the deal's line can't be clicked: %+v", students.Lines)
	}

	drawn = nil
	_, err = RenderDesignerPreview(DesignerInput{Mode: receiptModeClient, Deal: &DesignerDeal{Name: "Staff", Kind: "amount", Amount: 500},
		Texts: `{"elements":{"invoice_discount":{"text":"Deal: {name}","case":"upper"}}}`})
	if err != nil {
		t.Fatal(err)
	}
	if !drewText(drawn, "DEAL: STAFF") || !drewText(drawn, "-$5.00") || !drewText(drawn, "$235.80") {
		t.Fatalf("an amount deal with its own words: %q", drawn)
	}
	// The text receipt says the same.
	data := receiptPreviewData(designBase(), receiptModeClient)
	applyDesignerDeal(&data, DesignerDeal{Name: "Students 15%", Kind: "percent", Percent: 15})
	data.Texts = receiptTexts{Elements: map[string]receiptElement{"invoice_discount": {Text: "Saved with {name}"}}}
	if out := string(buildESCPosReceipt(data)); !strings.Contains(out, "Saved with Students 15%") {
		t.Fatalf("text receipt: %s", out)
	}
	// Paid in cash with $5 back, as before the deal.
	var paid, change int64
	for _, p := range data.Payments {
		if p.EntryType == "change" {
			change += p.AmountCents
		} else {
			paid += p.AmountCents
		}
	}
	if paid+change != data.GrandTotalCents || change != -500 {
		t.Fatalf("payments %d %d for %d", paid, change, data.GrandTotalCents)
	}
	// A deal bigger than the sale takes it to nothing, never below.
	data = receiptPreviewData(designBase(), receiptModeClient)
	applyDesignerDeal(&data, DesignerDeal{Name: "Huge", Kind: "amount", Amount: 99999999})
	if data.GrandTotalCents != 0 || data.InvoiceDiscount.Cents != 24080 {
		t.Fatalf("huge deal: %d %+v", data.GrandTotalCents, data.InvoiceDiscount)
	}
}

// The reported case: deals like buy 2 get 1 have a view, and the free unit's
// tag can be reworded or left off; untouched it is the till's own.
func TestDesignerItemDeal(t *testing.T) {
	var drawn []string
	saved := receiptTextDrawn
	receiptTextDrawn = func(v string, _ int) { drawn = append(drawn, v) }
	defer func() { receiptTextDrawn = saved }()
	render := func(texts string) []string {
		drawn = nil
		if _, err := RenderDesignerPreview(DesignerInput{Mode: receiptModeItemDeal, Texts: texts}); err != nil {
			t.Fatal(err)
		}
		return drawn
	}
	plain := render("")
	if !drewText(plain, "(FREE)") || !drewText(plain, "$85.00") {
		t.Fatalf("buy 2 get 1: %q", plain)
	}
	own := render(`{"elements":{"deal_free":{"text":"free gift","case":"upper"}}}`)
	if !drewText(own, "FREE GIFT") || drewText(own, "(FREE)") {
		t.Fatalf("own tag: %q", own)
	}
	// Hidden, the tag after the name goes; the amount still says FREE.
	off := render(`{"elements":{"deal_free":{"hide":true}}}`)
	if drewText(off, "(FREE)") || drewText(off, "Blue Linen Shirt (FREE)") || !drewText(off, "FREE") {
		t.Fatalf("hidden tag printed: %q", off)
	}
	// The text receipt follows, and an item that isn't free keeps its name.
	data := receiptPreviewData(designBase(), receiptModeItemDeal)
	data.Texts = receiptTexts{Elements: map[string]receiptElement{"deal_free": {Text: "on us"}}}
	out := string(buildESCPosReceipt(data))
	if !strings.Contains(out, "Blue Linen Shirt on us") || strings.Contains(out, "(FREE)") {
		t.Fatalf("text receipt: %s", out)
	}
	items := itemsWithFreeTag(receiptTexts{Elements: map[string]receiptElement{"deal_free": {Text: "x"}}}, []invoiceItem{{Name: "Hat (FREE)"}, {Name: "Cap (FREE)", Free: true}})
	if items[0].Name != "Hat (FREE)" || items[1].Name != "Cap x" {
		t.Fatalf("names %q %q", items[0].Name, items[1].Name)
	}
}
