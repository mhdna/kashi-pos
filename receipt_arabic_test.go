package main

import "testing"

func TestShapeArabic(t *testing.T) {
	cases := map[string]string{
		// ba alone; ba ba: initial then final; three: initial, medial, final.
		"ب":   "\uFE8F",
		"بب":  "\uFE91\uFE90",
		"ببب": "\uFE91\uFE92\uFE90",
		// alef joins only to the letter before: ba-alef-ba is initial, final,
		// then ba alone.
		"باب": "\uFE91\uFE8E\uFE8F",
		// lam-alef is one glyph; after a joining letter, its final form.
		"لا":  "\uFEFB",
		"بلا": "\uFE91\uFEFC",
		// vowel signs are left out; Latin passes through.
		"شُكراً": "\uFEB7\uFEDC\uFEAE\uFE8D",
		"Hi":     "Hi",
	}
	for in, want := range cases {
		if got := string(shapeArabic(in)); got != want {
			t.Errorf("%q: got %U, want %U", in, []rune(got), []rune(want))
		}
	}
}

func TestArabicVisualOrder(t *testing.T) {
	// No Arabic: untouched.
	if got := arabicVisual("Subtotal $85.00"); got != "Subtotal $85.00" {
		t.Fatalf("latin: %q", got)
	}
	// Arabic reads right to left: the last word is drawn first (leftmost).
	got := []rune(arabicVisual("بب لا"))
	if string(got) != "\uFEFB \uFE90\uFE91" {
		t.Fatalf("order: %U", got)
	}
	// A name, an amount and a percent inside keep their own order.
	for _, keep := range []string{"Maya Haddad", "$85.00", "15%", "3"} {
		v := arabicVisual("الزبون " + keep + " شكرا")
		if !containsRunes(v, keep) {
			t.Fatalf("%q broken in %q", keep, v)
		}
	}
	// Brackets face the right way once reversed.
	if v := arabicVisual("(مجانا)"); []rune(v)[0] != '(' || []rune(v)[len([]rune(v))-1] != ')' {
		t.Fatalf("brackets: %q", v)
	}
}

func containsRunes(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s[:len(sub)] == sub || containsRunes(s[1:], sub)))
}

func arTexts(elements map[string]receiptElement) receiptTexts {
	return normalizeReceiptTexts(receiptTexts{Elements: elements})
}

func TestArabicWords(t *testing.T) {
	t0 := arTexts(nil)
	// The English words swapped, what follows them kept.
	if got := t0.arabicWords("meta_customer", "Customer: Maya Haddad"); got != "الزبون: Maya Haddad" {
		t.Fatalf("customer: %q", got)
	}
	// The reported case: the referral note in Lebanese, its percent in Arabic digits.
	both := receiptTexts{ReferralRewardTo: referralRewardBoth}
	note := both.arabicWords("referral_note", "Share this code with someone who hasn't shopped with us yet. When they use it, you both get 15% off the whole invoice.")
	if note != "شارك هالكود مع حدا ما زارنا قبل، ولما يستعملو بتاخدوا انتو التنين حسم 15% على كامل الفاتورة." {
		t.Fatalf("note: %q", note)
	}
	// The shop's own policy has no default Arabic; the default policy does.
	if got := t0.arabicWords("policy_text", "No returns on sale items."); got != "" {
		t.Fatalf("own policy: %q", got)
	}
	// The reported case: Centro's policy starts with the default sentence;
	// its Arabic is the branch's alone, nothing of the English after it.
	centro := arTexts(map[string]receiptElement{"policy_text": {Lang: "both", TextAr: "فيك تبدل خلال ٣ أيام. جيب هيدي الفاتورة معك."}})
	if got := centro.arabicWords("policy_text", defaultReturnPolicy+" Items must be unworn, unused, with tags. Present this receipt."); got != "فيك تبدل خلال ٣ أيام. جيب هيدي الفاتورة معك." {
		t.Fatalf("centro policy: %q", got)
	}
	if got := t0.arabicWords("referral_title", "YOUR CODE: K7M4QH"); got != "الكود تبعك K7M4QH" && got != "الكود تبعك: K7M4QH" {
		t.Fatalf("referral title keeps its code: %q", got)
	}
	if got := t0.arabicWords("policy_text", defaultReturnPolicy); got != "فيك تبدل خلال ٣ أيام." {
		t.Fatalf("default policy: %q", got)
	}
	// The branch's own Arabic wins; a line with none (the store name) has none.
	own := arTexts(map[string]receiptElement{"policy_text": {Lang: "both", TextAr: "ما في ترجيع"}, "store_name": {Lang: "ar", TextAr: "بازار"}})
	if got := own.arabicWords("policy_text", "No returns on sale items."); got != "ما في ترجيع" {
		t.Fatalf("own arabic: %q", got)
	}
	if got := own.arabicWords("store_name", "Bazaar"); got != "بازار" {
		t.Fatalf("store name: %q", got)
	}
	if got := t0.arabicWords("store_name", "Bazaar"); got != "" {
		t.Fatalf("store name without arabic: %q", got)
	}
	// Cleaning keeps only known languages, on lines that can be Arabic.
	n := arTexts(map[string]receiptElement{"thanks": {Lang: "both"}, "subtotal": {Lang: "klingon"}, "col_item": {Lang: "ar"}})
	if n.Elements["thanks"].Lang != "both" || n.Elements["subtotal"].Lang != "" {
		t.Fatalf("clean: %+v", n.Elements)
	}
	if _, kept := n.Elements["col_item"]; kept {
		t.Fatal("a column title can't be Arabic yet")
	}
	if mirrorAlign("left") != "right" || mirrorAlign("right") != "left" || mirrorAlign("") != "center" {
		t.Fatal("mirror")
	}
}

// Every line that can be Arabic prints Arabic in place of its English
// ("ar") or under it ("both"), on every look; the text receipt stays
// English (its printer can't print Arabic).
func TestEveryArabicLineDraws(t *testing.T) {
	var keys []string
	for _, e := range receiptElements {
		if e.Can&canArabic != 0 && arabicDefaults[e.Key] != "" {
			keys = append(keys, e.Key)
		}
	}
	if len(keys) < 15 {
		t.Fatalf("only %d lines can be Arabic", len(keys))
	}
	countArabic := func(drawn []string) int {
		n := 0
		for _, d := range drawn {
			if hasArabic(d) {
				n++
			}
		}
		return n
	}
	for _, lang := range []string{receiptLangArabic, receiptLangBoth} {
		elements := map[string]receiptElement{}
		for _, k := range keys {
			elements[k] = receiptElement{Lang: lang}
		}
		for _, theme := range receiptThemeCatalog() {
			for _, mode := range []string{receiptModeReferral, receiptModeSale} {
				data := receiptPreviewData(designBase(), mode)
				data.Texts = arTexts(elements)
				data.ReferralNote = data.Texts.referralNote(sampleReferralPercent)
				drawn, img, marks := drawnTexts(t, data, theme.ID, theme.Layouts[0].ID)
				if countArabic(drawn) < 6 {
					t.Fatalf("%s %s %s: %d Arabic lines drawn: %q", lang, theme.ID, mode, countArabic(drawn), drawn)
				}
				for _, m := range marks {
					if m.Bottom > img.Bounds().Dy() || m.Top < 0 {
						t.Fatalf("%s %s: %s off the paper", lang, theme.ID, m.Part)
					}
				}
				all := " " + joinDrawn(drawn) + " "
				for _, en := range []string{"Thank you for shopping with us!", "EXCHANGE POLICY"} {
					if lang == receiptLangArabic && containsWord(all, en) {
						t.Fatalf("%s %s: still prints %q", lang, theme.ID, en)
					}
				}
				if lang == receiptLangBoth && mode == receiptModeSale && !containsWord(all, "Thank you for shopping with us!") && theme.ID == "classic" {
					t.Fatalf("both: the English line went")
				}
				if text := string(buildESCPosReceipt(data)); hasArabic(text) {
					t.Fatalf("the text receipt printed Arabic")
				}
			}
		}
	}
}

func joinDrawn(d []string) string {
	out := ""
	for _, s := range d {
		out += s + "\n"
	}
	return out
}

func containsWord(all, w string) bool {
	for i := 0; i+len(w) <= len(all); i++ {
		if all[i:i+len(w)] == w {
			return true
		}
	}
	return false
}
