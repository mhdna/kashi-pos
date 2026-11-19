package main

import (
	"strings"
	"unicode"
)

// Arabic on the receipt. The receipt's text drawing places one glyph per
// character, left to right, with no shaping, so Arabic is prepared here:
// each letter takes its joined form (alone, start, middle or end of a word,
// from Unicode's Arabic Presentation Forms-B, which the receipt's Arabic
// fonts carry), lam-alef becomes its ligature, and the line is put in the
// order it is seen, right to left, with numbers and Latin words inside it
// kept left to right.

// arabicForms are a letter's isolated, final, initial and medial forms; a
// letter that joins only to the one before it (alef, dal, ra, waw...) has no
// initial or medial form.
var arabicForms = map[rune][4]rune{
	0x0621: {0xFE80, 0, 0, 0},
	0x0622: {0xFE81, 0xFE82, 0, 0},
	0x0623: {0xFE83, 0xFE84, 0, 0},
	0x0624: {0xFE85, 0xFE86, 0, 0},
	0x0625: {0xFE87, 0xFE88, 0, 0},
	0x0626: {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C},
	0x0627: {0xFE8D, 0xFE8E, 0, 0},
	0x0628: {0xFE8F, 0xFE90, 0xFE91, 0xFE92},
	0x0629: {0xFE93, 0xFE94, 0, 0},
	0x062A: {0xFE95, 0xFE96, 0xFE97, 0xFE98},
	0x062B: {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C},
	0x062C: {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0},
	0x062D: {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4},
	0x062E: {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8},
	0x062F: {0xFEA9, 0xFEAA, 0, 0},
	0x0630: {0xFEAB, 0xFEAC, 0, 0},
	0x0631: {0xFEAD, 0xFEAE, 0, 0},
	0x0632: {0xFEAF, 0xFEB0, 0, 0},
	0x0633: {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4},
	0x0634: {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8},
	0x0635: {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC},
	0x0636: {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0},
	0x0637: {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4},
	0x0638: {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8},
	0x0639: {0xFEC9, 0xFECA, 0xFECB, 0xFECC},
	0x063A: {0xFECD, 0xFECE, 0xFECF, 0xFED0},
	0x0641: {0xFED1, 0xFED2, 0xFED3, 0xFED4},
	0x0642: {0xFED5, 0xFED6, 0xFED7, 0xFED8},
	0x0643: {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC},
	0x0644: {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0},
	0x0645: {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4},
	0x0646: {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8},
	0x0647: {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC},
	0x0648: {0xFEED, 0xFEEE, 0, 0},
	0x0649: {0xFEEF, 0xFEF0, 0, 0},
	0x064A: {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4},
}

// lamAlef are lam + an alef as one glyph: isolated, final.
var lamAlef = map[rune][2]rune{
	0x0622: {0xFEF5, 0xFEF6},
	0x0623: {0xFEF7, 0xFEF8},
	0x0625: {0xFEF9, 0xFEFA},
	0x0627: {0xFEFB, 0xFEFC},
}

const (
	arabicLam     = 0x0644
	arabicTatweel = 0x0640
)

// arabicMark is a vowel sign (harakat): the receipt leaves them out, as its
// drawing can't place them over the letters.
func arabicMark(r rune) bool { return (r >= 0x064B && r <= 0x065F) || r == 0x0670 }

// joinsNext says whether a letter connects to the letter after it.
func joinsNext(r rune) bool {
	if r == arabicTatweel {
		return true
	}
	f, ok := arabicForms[r]
	return ok && f[2] != 0
}

// joinsPrev says whether a letter connects to the letter before it.
func joinsPrev(r rune) bool {
	if r == arabicTatweel {
		return true
	}
	f, ok := arabicForms[r]
	return ok && f[1] != 0
}

// shapeArabic gives each Arabic letter its joined form, in logical order.
func shapeArabic(s string) []rune {
	var in []rune
	for _, r := range s {
		if !arabicMark(r) {
			in = append(in, r)
		}
	}
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		r := in[i]
		forms, ok := arabicForms[r]
		if !ok {
			out = append(out, r)
			continue
		}
		prev := i > 0 && joinsNext(in[i-1])
		if r == arabicLam && i+1 < len(in) {
			if lig, ok := lamAlef[in[i+1]]; ok {
				if prev {
					out = append(out, lig[1])
				} else {
					out = append(out, lig[0])
				}
				i++
				continue
			}
		}
		next := i+1 < len(in) && joinsPrev(in[i+1]) && joinsNext(r)
		switch {
		case prev && forms[1] != 0 && next:
			out = append(out, forms[3])
		case prev && forms[1] != 0:
			out = append(out, forms[1])
		case next:
			out = append(out, forms[2])
		default:
			out = append(out, forms[0])
		}
	}
	return out
}

// hasArabic says whether s holds any Arabic letter.
func hasArabic(s string) bool {
	for _, r := range s {
		if isArabicRune(r) {
			return true
		}
	}
	return false
}

func isArabicRune(r rune) bool {
	return (r >= 0x0600 && r <= 0x06FF) || (r >= 0xFB50 && r <= 0xFDFF) || (r >= 0xFE70 && r <= 0xFEFF)
}

// bidiClass is a character's direction: 'R' Arabic, 'L' Latin letters and
// digits, 'N' neutral (spaces, punctuation) taking its neighbours'.
func bidiClass(r rune) byte {
	switch {
	case isArabicRune(r) && !(r >= 0x0660 && r <= 0x0669):
		return 'R'
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 'L'
	}
	return 'N'
}

var mirrored = map[rune]rune{'(': ')', ')': '(', '[': ']', ']': '[', '{': '}', '}': '{', '<': '>', '>': '<', '«': '»', '»': '«'}

// arabicVisual is an Arabic line as it is drawn left to right: joined
// letters, the words right to left, and runs of Latin words or numbers kept
// in their own order. A line with no Arabic is returned as it is.
func arabicVisual(s string) string {
	if !hasArabic(s) {
		return s
	}
	shaped := shapeArabic(s)
	classes := make([]byte, len(shaped))
	for i, r := range shaped {
		classes[i] = bidiClass(r)
	}
	// A neutral between two left-to-right characters joins them ("$85.00",
	// "Maya Haddad"); any other neutral reads right to left.
	for i := range classes {
		if classes[i] != 'N' {
			continue
		}
		var before, after byte = 'R', 'R'
		for j := i - 1; j >= 0; j-- {
			if classes[j] != 'N' {
				before = classes[j]
				break
			}
		}
		for j := i + 1; j < len(classes); j++ {
			if classes[j] != 'N' {
				after = classes[j]
				break
			}
		}
		if before == 'L' && after == 'L' {
			classes[i] = 'l'
		}
	}
	// A currency sign or sign before a number goes with it ("$85.00",
	// "-5"), and a percent after one ("15%").
	for i, r := range shaped {
		if classes[i] != 'N' {
			continue
		}
		if strings.ContainsRune("$€£+-", r) && i+1 < len(shaped) && (classes[i+1] == 'L' || classes[i+1] == 'l') {
			classes[i] = 'l'
		}
		if r == '%' && i > 0 && (classes[i-1] == 'L' || classes[i-1] == 'l') {
			classes[i] = 'l'
		}
	}
	type run struct {
		ltr   bool
		runes []rune
	}
	var runs []run
	for i, r := range shaped {
		ltr := classes[i] == 'L' || classes[i] == 'l'
		if len(runs) == 0 || runs[len(runs)-1].ltr != ltr {
			runs = append(runs, run{ltr: ltr})
		}
		runs[len(runs)-1].runes = append(runs[len(runs)-1].runes, r)
	}
	var b strings.Builder
	for i := len(runs) - 1; i >= 0; i-- {
		rr := runs[i].runes
		if runs[i].ltr {
			b.WriteString(string(rr))
			continue
		}
		for j := len(rr) - 1; j >= 0; j-- {
			r := rr[j]
			if m, ok := mirrored[r]; ok {
				r = m
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
