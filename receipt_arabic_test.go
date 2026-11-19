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
