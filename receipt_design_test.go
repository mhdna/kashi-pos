package main

import (
	"encoding/json"
	"image"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// drawnTexts renders data and returns every piece of text it drew.
func drawnTexts(t *testing.T, data printReceiptData, theme, layout string) ([]string, image.Image, []receiptMark) {
	t.Helper()
	var texts []string
	saved := receiptTextDrawn
	receiptTextDrawn = func(v string, _ int) { texts = append(texts, v) }
	defer func() { receiptTextDrawn = saved }()
	png, marks, err := renderReceiptPreviewMarked(data, theme, layout)
	if err != nil {
		t.Fatal(err)
	}
	return texts, decodePNG(t, png), marks
}

func designBase() printReceiptData {
	return sampleSocials(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza", StoreName: "Bazaar"})
}

func withElements(data printReceiptData, elements map[string]receiptElement) printReceiptData {
	data.Texts.Elements = elements
	return data
}

func drewText(texts []string, want string) bool {
	for _, s := range texts {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

// The reported case: every label reworded, on every look.
func TestReceiptElementsReword(t *testing.T) {
	elements := map[string]receiptElement{
		"store_name":         {Text: "MY SHOP"},
		"invoice_label_sale": {Text: "Receipt"},
		"meta_customer":      {Text: "Client"},
		"meta_salesperson":   {Text: "Served by"},
		"meta_date":          {Text: "Day"},
		"col_item":           {Text: "Article", Case: "title"},
		"col_qty":            {Text: "Pcs"},
		"col_total":          {Text: "Sum"},
		"subtotal":           {Text: "Before discount"},
		"discount":           {Text: "Saved"},
		"total":              {Text: "To pay"},
		"equivalent":         {Text: "In {currency}"},
		"payments_title":     {Text: "How you paid"},
		"received":           {Text: "Paid"},
		"change":             {Text: "Back"},
		"policy_title":       {Text: "Swaps"},
		"policy_text":        {Text: "No refunds, swaps within 7 days."},
		"socials_title":      {Text: "Find us:"},
		"thanks":             {Text: "see you soon", Case: "upper"},
	}
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			texts, _, _ := drawnTexts(t, withElements(receiptPreviewData(designBase(), receiptModeSale), elements), theme.ID, layout.ID)
			for _, want := range []string{"MY SHOP", "Receipt: PV-10452", "Before discount", "Saved", "To pay", "In LBP", "How you paid", "Paid - Cash", "Back - Cash", "Swaps", "No refunds,", "Find us:", "SEE YOU SOON"} {
				if !drewText(texts, want) {
					t.Fatalf("%s: %q not printed in %q", where, want, texts)
				}
			}
			if !drewText(texts, "Client") || !drewText(texts, "Day") {
				t.Fatalf("%s: meta labels not reworded: %q", where, texts)
			}
			for _, gone := range []string{"Bazaar", "Sales Invoice", "Subtotal", "LBP equivalent", "PAYMENT DETAILS", "Received - ", "EXCHANGE POLICY", "Exchanges accepted", "Follow us on:", "Thank you"} {
				if drewText(texts, gone) {
					t.Fatalf("%s: default %q still printed", where, gone)
				}
			}
		}
	}
}

func TestReceiptElementsColumnTitles(t *testing.T) {
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			texts, _, _ := drawnTexts(t, withElements(receiptPreviewData(designBase(), receiptModeSale), map[string]receiptElement{
				"col_item": {Text: "Article"}, "col_qty": {Text: "Pcs"}, "col_disc": {Text: "Off"}, "col_price": {Text: "Each"},
				"col_total": {Text: "Sum"}, "col_amount": {Text: "Sum"},
			}), theme.ID, layout.ID)
			for _, gone := range []string{"ITEM", "QTY", "DISC", "AMOUNT"} {
				for _, s := range texts {
					if s == gone {
						t.Fatalf("%s/%s: column title %q still printed", theme.ID, layout.ID, gone)
					}
				}
			}
		}
	}
}

func TestReceiptCase(t *testing.T) {
	cases := map[string][2]string{
		"upper": {"thank you", "THANK YOU"},
		"lower": {"PAYMENT DETAILS", "payment details"},
		"title": {"PAYMENT details for-you", "Payment Details For-You"},
		"":      {"As Typed", "As Typed"},
		"bogus": {"As Typed", "As Typed"},
	}
	for c, io := range cases {
		if got := applyReceiptCase(io[0], c); got != io[1] {
			t.Fatalf("%q: %q, want %q", c, got, io[1])
		}
	}
	if got := applyReceiptCase("شكراً لكم", "upper"); got != "شكراً لكم" {
		t.Fatalf("arabic changed: %q", got)
	}
	// Capitals apply to the default words too.
	texts, _, _ := drawnTexts(t, withElements(receiptPreviewData(designBase(), receiptModeSale), map[string]receiptElement{"payments_title": {Case: "title"}}), "", "")
	if !drewText(texts, "Payment Details") {
		t.Fatalf("title case not applied to the default: %q", texts)
	}
}

func TestReceiptDateFormats(t *testing.T) {
	want := map[string]string{
		"": "19-10-2026 14:30", "slash": "19/10/2026 14:30", "slash12": "19/10/2026 2:30 PM",
		"iso": "2026-10-19 14:30", "long": "19 Oct 2026, 14:30", "longday": "Mon 19 Oct 2026, 14:30", "date": "19/10/2026",
	}
	for _, f := range receiptDateFormats {
		if f.Label != want[f.Key] {
			t.Fatalf("%q labelled %q, want %q", f.Key, f.Label, want[f.Key])
		}
		data := receiptPreviewData(designBase(), receiptModeSale)
		data.Texts.DateFormat = f.Key
		texts, _, _ := drawnTexts(t, data, "", "")
		if !drewText(texts, want[f.Key]) {
			t.Fatalf("%q: date %q not printed: %q", f.Key, want[f.Key], texts)
		}
	}
	if (receiptTexts{DateFormat: "nonsense"}).dateLayout() != receiptDateFormats[0].Layout {
		t.Fatal("unknown format isn't the default")
	}
}

func bandInk(img image.Image, top, bottom int) (left, right, count int) {
	left, right = img.Bounds().Dx(), -1
	for y := top; y < bottom && y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
				count++
				if x < left {
					left = x
				}
				if x > right {
					right = x
				}
			}
		}
	}
	return
}

// groupBand is everything a group covers, from its first line to its last.
func groupBand(t *testing.T, marks []receiptMark, group string) receiptMark {
	t.Helper()
	out, found := receiptMark{Top: 1 << 30}, false
	for _, m := range marks {
		if m.Kind == "group" && m.Part == group {
			found = true
			out.Top, out.Bottom = minInt(out.Top, m.Top), maxInt(out.Bottom, m.Bottom)
		}
	}
	if !found {
		t.Fatalf("no %s band", group)
	}
	return out
}

// Size, weight, alignment and hiding change what is printed, on every look.
func TestReceiptElementsStyle(t *testing.T) {
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			where := theme.ID + "/" + layout.ID
			plain := receiptPreviewData(designBase(), receiptModeSale)
			_, base, baseMarks := drawnTexts(t, plain, theme.ID, layout.ID)

			_, bigger, _ := drawnTexts(t, withElements(plain, map[string]receiptElement{"thanks": {Size: 6}}), theme.ID, layout.ID)
			if bigger.Bounds().Dy() <= base.Bounds().Dy() {
				t.Fatalf("%s: a bigger thank-you line isn't taller", where)
			}

			yes, no := true, false
			_, bold, boldMarks := drawnTexts(t, withElements(plain, map[string]receiptElement{"policy_text": {Bold: &yes}}), theme.ID, layout.ID)
			b := groupBand(t, boldMarks, "policy")
			p := groupBand(t, baseMarks, "policy")
			_, _, boldInk := bandInk(bold, b.Top, b.Bottom)
			_, _, plainInk := bandInk(base, p.Top, p.Bottom)
			if boldInk <= plainInk {
				t.Fatalf("%s: bold policy text has no more ink (%d vs %d)", where, boldInk, plainInk)
			}
			_, light, lightMarks := drawnTexts(t, withElements(plain, map[string]receiptElement{"payments_title": {Bold: &no}}), theme.ID, layout.ID)
			lp := groupBand(t, lightMarks, "payments")
			bp := groupBand(t, baseMarks, "payments")
			_, _, lightInk := bandInk(light, lp.Top, lp.Bottom)
			_, _, payInk := bandInk(base, bp.Top, bp.Bottom)
			if lightInk >= payInk {
				t.Fatalf("%s: the payment title not bold has no less ink", where)
			}

			_, right, rightMarks := drawnTexts(t, withElements(plain, map[string]receiptElement{"thanks": {Align: "right"}}), theme.ID, layout.ID)
			_, left, leftMarks := drawnTexts(t, withElements(plain, map[string]receiptElement{"thanks": {Align: "left"}}), theme.ID, layout.ID)
			rb, lb := groupBand(t, rightMarks, "thanks"), groupBand(t, leftMarks, "thanks")
			rl, rr, _ := bandInk(right, rb.Top, rb.Bottom)
			ll, lr, _ := bandInk(left, lb.Top, lb.Bottom)
			if !(rl > ll && rr > lr) {
				t.Fatalf("%s: right-aligned thanks (%d-%d) isn't right of left-aligned (%d-%d)", where, rl, rr, ll, lr)
			}
			if ll > 40 || rr < right.Bounds().Dx()-40 {
				t.Fatalf("%s: aligned thanks not at the edges: left starts %d, right ends %d", where, ll, rr)
			}

			for _, key := range []string{"store_name", "branch_name", "phone", "invoice_number", "subtotal", "socials_handle", "thanks", "meta_date"} {
				texts, hidden, _ := drawnTexts(t, withElements(plain, map[string]receiptElement{key: {Hide: true}}), theme.ID, layout.ID)
				if hidden.Bounds().Dy() >= base.Bounds().Dy() {
					t.Fatalf("%s: hiding %s doesn't shorten the receipt", where, key)
				}
				if key == "subtotal" && drewText(texts, "Subtotal") {
					t.Fatalf("%s: hidden subtotal printed", where)
				}
			}
		}
	}
}

// What the till keeps of a design: only known elements, each change within
// what that element allows.
func TestNormalizeReceiptElements(t *testing.T) {
	yes := true
	in := receiptTexts{
		DateFormat: "nope",
		Elements: map[string]receiptElement{
			"thanks":   {Text: "  Bye  ", Case: "upper", Size: 30, Bold: &yes, Align: "right", Hide: true},
			"col_item": {Text: "Article", Size: 4, Bold: &yes, Align: "center", Hide: true, Case: "shout"},
			"nonsense": {Text: "x"},
			"total":    {},
			"phone":    {Size: -20.3},
			"subtotal": {Size: 1.26},
		},
	}
	out := normalizeReceiptTexts(in)
	if out.DateFormat != "" {
		t.Fatalf("bad date format kept: %q", out.DateFormat)
	}
	if got := out.Elements["thanks"]; got.Text != "Bye" || got.Case != "upper" || got.Size != receiptElementMaxSize || got.Bold == nil || got.Align != "right" || !got.Hide {
		t.Fatalf("thanks: %+v", got)
	}
	if got := out.Elements["col_item"]; got != (receiptElement{Text: "Article"}) {
		t.Fatalf("a column title keeps only its words and capitals: %+v", got)
	}
	if _, ok := out.Elements["nonsense"]; ok {
		t.Fatal("unknown element kept")
	}
	if _, ok := out.Elements["total"]; ok {
		t.Fatal("an element with nothing set kept")
	}
	if out.Elements["phone"].Size != receiptElementMinSize {
		t.Fatalf("size not clamped: %v", out.Elements["phone"].Size)
	}
	if _, ok := out.Elements["subtotal"]; ok {
		t.Fatal("subtotal can't change size")
	}
	raw, _ := json.Marshal(out)
	var back receiptTexts
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if again, _ := json.Marshal(normalizeReceiptTexts(back)); string(again) != string(raw) {
		t.Fatalf("not stable:\n%s\n%s", raw, again)
	}
}

// The wording saved before elements existed still prints.
func TestReceiptLegacyTextsStillPrint(t *testing.T) {
	data := receiptPreviewData(designBase(), receiptModeSale)
	data.Texts = receiptTexts{PaymentsTitle: "PAID WITH", Thanks: "See you"}
	texts, _, _ := drawnTexts(t, data, "", "")
	if !drewText(texts, "PAID WITH") || !drewText(texts, "See you") {
		t.Fatalf("old wording lost: %q", texts)
	}
	data.Texts.Elements = map[string]receiptElement{"thanks": {Text: "Bye"}}
	texts, _, _ = drawnTexts(t, data, "", "")
	if !drewText(texts, "Bye") || drewText(texts, "See you") {
		t.Fatalf("an element's words come before the old field: %q", texts)
	}
}

// The text receipt takes the same words.
func TestESCPosReceiptElements(t *testing.T) {
	data := receiptPreviewData(designBase(), receiptModeSale)
	data.Texts.Elements = map[string]receiptElement{
		"col_item": {Text: "Article"}, "total": {Text: "To pay"}, "subtotal": {Hide: true},
		"equivalent": {Text: "In {currency}"}, "meta_customer": {Text: "Client"},
	}
	out := string(buildESCPosReceipt(data))
	for _, want := range []string{"Article", "To pay", "In LBP", "Client:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("text receipt lacks %q", want)
		}
	}
	if strings.Contains(out, "Subtotal") {
		t.Fatal("hidden subtotal printed")
	}
}

// Randomized: any design the designer can send draws on every look and
// mode, its parts inside the paper, and cleaning it twice changes nothing.
func TestReceiptDesignsRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	words := []string{"", "Hi", "  spaced  ", "LONG " + strings.Repeat("word ", 20), "شكراً", "{percent} off", "{currency}"}
	cases := []string{"", "upper", "lower", "title", "junk"}
	aligns := []string{"", "left", "center", "right", "middle"}
	themes := receiptThemeCatalog()
	for run := 0; run < 120; run++ {
		elements := map[string]receiptElement{}
		for _, info := range receiptElements {
			if rng.Intn(3) != 0 {
				continue
			}
			b := rng.Intn(2) == 0
			elements[info.Key] = receiptElement{
				Text: words[rng.Intn(len(words))], Case: cases[rng.Intn(len(cases))],
				Size: float64(rng.Intn(30) - 10), Bold: &b, Align: aligns[rng.Intn(len(aligns))], Hide: rng.Intn(5) == 0,
			}
		}
		texts := normalizeReceiptTexts(receiptTexts{Elements: elements, DateFormat: receiptDateFormats[rng.Intn(len(receiptDateFormats))].Key})
		raw, _ := json.Marshal(texts)
		var back receiptTexts
		_ = json.Unmarshal(raw, &back)
		if again, _ := json.Marshal(normalizeReceiptTexts(back)); string(again) != string(raw) {
			t.Fatalf("cleaning twice changed it:\n%s\n%s", raw, again)
		}
		theme := themes[rng.Intn(len(themes))]
		layout := theme.Layouts[rng.Intn(len(theme.Layouts))]
		mode := receiptPreviewModes[rng.Intn(len(receiptPreviewModes))]
		data := receiptPreviewData(designBase(), mode)
		data.Texts = texts
		data.ReferralNote = texts.referralNote(sampleReferralPercent)
		_, img, marks := drawnTexts(t, data, theme.ID, layout.ID)
		for _, m := range marks {
			if m.Top < 0 || m.Bottom > img.Bounds().Dy() || m.Top > m.Bottom {
				t.Fatalf("%s/%s/%s: %s band %d-%d outside 0-%d (%s)", theme.ID, layout.ID, mode, m.Part, m.Top, m.Bottom, img.Bounds().Dy(), raw)
			}
		}
	}
}
