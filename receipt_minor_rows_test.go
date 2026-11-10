package main

import (
	"testing"
)

type tracedReceiptText struct {
	size float64
	role fontRole
}

// traceReceiptText renders data in the given look and returns how each
// string was drawn (the last time it was drawn, if more than once).
func traceReceiptText(t *testing.T, data printReceiptData, themeID, layoutID string) map[string]tracedReceiptText {
	t.Helper()
	drawn := map[string]tracedReceiptText{}
	receiptTextTrace = func(value string, size float64, role fontRole) {
		drawn[value] = tracedReceiptText{size, role}
	}
	defer func() { receiptTextTrace = nil }()
	if _, err := renderReceiptPreview(data, themeID, layoutID); err != nil {
		t.Fatalf("%s/%s: %v", themeID, layoutID, err)
	}
	return drawn
}

func isBoldRole(role fontRole) bool {
	return role == roleBodyBold || role == roleTitleBold || role == roleName
}

// A sale with a discount, an invoice discount, a rounding leftover and cash
// change, so every secondary line around the total prints.
func receiptWithEverySummaryLine() printReceiptData {
	base := printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"}
	data := sampleReceiptData(base)
	// Snap up to the next whole dollar, as the cash grid does, so the
	// leftover prints as a Rounding line.
	data.GrandTotalCents = (data.GrandTotalCents/100 + 1) * 100
	data.Payments = []receiptPayment{
		{AccountName: "Cash", CurrencyCode: "USD", AmountCents: data.GrandTotalCents + 500, EntryType: "payment"},
		{AccountName: "Cash", CurrencyCode: "USD", AmountCents: -500, EntryType: "change"},
	}
	return data
}

// Subtotal, Discount, Rounding and the payment lines are working lines: they
// print in the regular weight and a size below the TOTAL, which alone stays
// bold, in every theme and layout.
func TestReceiptSummaryLinesAreLighterThanTotal(t *testing.T) {
	minor := []string{"Subtotal", "Discount", "Rounding", "Received - Cash", "Change - Cash"}
	for _, theme := range receiptThemeCatalog() {
		for _, layout := range theme.Layouts {
			drawn := traceReceiptText(t, receiptWithEverySummaryLine(), theme.ID, layout.ID)
			total, ok := drawn["TOTAL"]
			if !ok {
				t.Fatalf("%s/%s: TOTAL not drawn", theme.ID, layout.ID)
			}
			if !isBoldRole(total.role) {
				t.Errorf("%s/%s: TOTAL drawn in role %v, want bold", theme.ID, layout.ID, total.role)
			}
			for _, label := range minor {
				got, ok := drawn[label]
				if !ok {
					t.Errorf("%s/%s: %q not drawn", theme.ID, layout.ID, label)
					continue
				}
				if isBoldRole(got.role) {
					t.Errorf("%s/%s: %q drawn bold, want regular", theme.ID, layout.ID, label)
				}
				if got.size != receiptMinorSize {
					t.Errorf("%s/%s: %q drawn at %.1f, want %d", theme.ID, layout.ID, label, got.size, receiptMinorSize)
				}
				if got.size >= total.size {
					t.Errorf("%s/%s: %q (%.1f) not smaller than TOTAL (%.1f)", theme.ID, layout.ID, label, got.size, total.size)
				}
			}
		}
	}
}

// A sale with nothing to round prints no Rounding line at all.
func TestReceiptOmitsRoundingWhenExact(t *testing.T) {
	base := printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"}
	drawn := traceReceiptText(t, sampleReceiptData(base), defaultReceiptTheme, defaultReceiptLayout)
	if _, ok := drawn["Rounding"]; ok {
		t.Error("Rounding printed for a sale with no rounding")
	}
	if _, ok := drawn["Subtotal"]; !ok {
		t.Error("Subtotal missing")
	}
}
