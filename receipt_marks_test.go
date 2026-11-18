package main

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func markFor(marks []receiptMark, part string) (receiptMark, bool) {
	return markOf(marks, "", part)
}

// markOf finds the mark of a kind ("" any) for a part. Sections come first
// when the kind isn't given, as "thanks" is both a section and a text.
func markOf(marks []receiptMark, kind, part string) (receiptMark, bool) {
	if kind == "" {
		if m, ok := markOf(marks, "section", part); ok {
			return m, true
		}
	}
	for _, m := range marks {
		if m.Part == part && (kind == "" || m.Kind == kind) {
			return m, true
		}
	}
	return receiptMark{}, false
}

func renderMarked(t *testing.T, data printReceiptData, theme, layout string) ([]byte, []receiptMark) {
	t.Helper()
	img, marks, err := renderReceiptPreviewMarked(data, theme, layout)
	if err != nil {
		t.Fatal(err)
	}
	return img, marks
}

// sameRows reports whether a's rows [aFrom, aFrom+n) equal b's [bFrom, bFrom+n).
func sameRows(a, b image.Image, aFrom, bFrom, n int) bool {
	w := a.Bounds().Dx()
	if b.Bounds().Dx() != w {
		return false
	}
	for dy := 0; dy < n; dy++ {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := a.At(x, aFrom+dy).RGBA()
			r2, g2, b2, _ := b.At(x, bFrom+dy).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				return false
			}
		}
	}
	return true
}

// Central's designer closes a part's band when it is left out. That is only
// right if the till, leaving the part out, prints exactly the full receipt
// with that band cut: every theme and layout, every section.
func TestReceiptMarksHidingCutsTheBand(t *testing.T) {
	base := sampleSocials(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			full := receiptPreviewData(base, receiptModeReferral)
			fullPNG, marks := renderMarked(t, full, theme.ID, layout.ID)
			fullImg := decodePNG(t, fullPNG)
			for _, section := range receiptSections {
				band, ok := markFor(marks, section)
				if !ok {
					t.Fatalf("%s/%s: no %s band", theme.ID, layout.ID, section)
				}
				hidden := full
				hidden.Texts = receiptTexts{Hide: []string{section}}
				if section == receiptSectionReferral {
					// referralReceiptNote prints no code for a hidden block.
					hidden.ReferralCode, hidden.ReferralNote = "", ""
				}
				hiddenPNG, _ := renderMarked(t, hidden, theme.ID, layout.ID)
				hiddenImg := decodePNG(t, hiddenPNG)
				cut := band.Bottom - band.Top
				if got, want := hiddenImg.Bounds().Dy(), fullImg.Bounds().Dy()-cut; got != want {
					t.Fatalf("%s/%s hiding %s: %d rows, want %d (full %d less the band %d-%d)", theme.ID, layout.ID, section, got, want, fullImg.Bounds().Dy(), band.Top, band.Bottom)
				}
				if !sameRows(fullImg, hiddenImg, 0, 0, band.Top) {
					t.Fatalf("%s/%s hiding %s: the receipt above the band changed", theme.ID, layout.ID, section)
				}
				if !sameRows(fullImg, hiddenImg, band.Bottom, band.Top, fullImg.Bounds().Dy()-band.Bottom) {
					t.Fatalf("%s/%s hiding %s: the receipt below the band is not the rest moved up", theme.ID, layout.ID, section)
				}
			}
		}
	}
}

// Every mode draws; its parts sit inside the image, top to bottom, each text
// inside its section; the referral block is only on the referral receipt.
func TestReceiptMarksPerMode(t *testing.T) {
	base := sampleSocials(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
	inside := map[string]string{
		receiptPartPaymentsTitle: receiptSectionPayments, receiptPartPolicyTitle: receiptSectionPolicy,
		receiptPartReferralTitle: receiptSectionReferral, receiptPartReferralNote: receiptSectionReferral,
		receiptPartSocialsTitle: receiptSectionSocials, receiptPartSocialsIcons: receiptSectionSocials,
	}
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			for _, mode := range receiptPreviewModes {
				img, marks := renderMarked(t, receiptPreviewData(base, mode), theme.ID, layout.ID)
				height := decodePNG(t, img).Bounds().Dy()
				where := theme.ID + "/" + layout.ID + "/" + mode
				for _, m := range marks {
					if m.Top < 0 || m.Bottom > height || m.Top >= m.Bottom {
						t.Fatalf("%s: %s band %d-%d outside 0-%d", where, m.Part, m.Top, m.Bottom, height)
					}
					if section, ok := inside[m.Part]; ok {
						outer, _ := markFor(marks, section)
						if m.Top < outer.Top || m.Bottom > outer.Bottom {
							t.Fatalf("%s: %s %d-%d outside its %s %d-%d", where, m.Part, m.Top, m.Bottom, section, outer.Top, outer.Bottom)
						}
					}
				}
				for _, part := range []string{receiptSectionPayments, receiptPartPaymentsTitle, receiptSectionPolicy, receiptPartPolicyTitle, receiptSectionSocials, receiptPartSocialsTitle, receiptPartSocialsIcons, receiptPartThanks} {
					if _, ok := markFor(marks, part); !ok {
						t.Fatalf("%s: no %s", where, part)
					}
				}
				_, hasReferral := markFor(marks, receiptSectionReferral)
				if hasReferral != (mode == receiptModeReferral) {
					t.Fatalf("%s: referral block shown %v", where, hasReferral)
				}
				// Sections follow one another down the paper.
				last := 0
				for _, section := range receiptSections {
					if m, ok := markFor(marks, section); ok {
						if m.Top < last {
							t.Fatalf("%s: %s starts at %d, above the part before (%d)", where, section, m.Top, last)
						}
						last = m.Bottom
					}
				}
			}
		}
	}
}

// A text part carries the words actually printed, so central can tell what
// it is replacing.
func TestReceiptMarksCarryTheWording(t *testing.T) {
	base := sampleSocials(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11})
	data := receiptPreviewData(base, receiptModeReferral)
	data.Texts = receiptTexts{PaymentsTitle: "PAID WITH", PolicyTitle: "SWAPS", ReferralTitle: "SHARE:", SocialsTitle: "Find us:", Thanks: "See you"}
	data.ReferralNote = data.Texts.referralNote(20)
	_, marks := renderMarked(t, data, "", "")
	want := map[string]string{
		receiptPartPaymentsTitle: "PAID WITH", receiptPartPolicyTitle: "SWAPS", receiptPartReferralTitle: "SHARE:",
		receiptPartSocialsTitle: "Find us:", receiptPartThanks: "See you",
	}
	for part, text := range want {
		m, _ := markOf(marks, "text", part)
		if m.Text != text || m.SizePx <= 0 {
			t.Fatalf("%s: %q at %.1fpx, want %q", part, m.Text, m.SizePx, text)
		}
	}
	if m, _ := markFor(marks, receiptPartReferralNote); !strings.Contains(m.Text, "20%") {
		t.Fatalf("referral note %q", m.Text)
	}
}

// The modes are the sale central's designer shows: the sums add up.
func TestReceiptPreviewModesAddUp(t *testing.T) {
	base := printReceiptData{}
	for _, mode := range receiptPreviewModes {
		data := receiptPreviewData(base, mode)
		var total float64
		for _, item := range data.Items {
			gross, discount, _ := itemAmounts(item)
			total += gross - discount
		}
		if got := int64(total*100 + sign(total)*0.5); got != data.GrandTotalCents {
			t.Fatalf("%s: lines come to %d, total says %d", mode, got, data.GrandTotalCents)
		}
		var paid int64
		for _, p := range data.Payments {
			paid += p.AmountCents
		}
		if mode == receiptModeSale || mode == receiptModeReferral || mode == receiptModeClient {
			continue // cash given and change back
		}
		if paid != data.GrandTotalCents {
			t.Fatalf("%s: paid %d of %d", mode, paid, data.GrandTotalCents)
		}
	}
	if receiptInvoiceLabel(receiptPreviewData(base, receiptModeExchange).ReceiptNo) != "Exchange Invoice" ||
		receiptInvoiceLabel(receiptPreviewData(base, receiptModeReturn).ReceiptNo) != "Return Invoice" {
		t.Fatal("exchange and return samples aren't labelled as such")
	}
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
