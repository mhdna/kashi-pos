package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateReceiptPreviews writes one PNG per theme and layout, plus a
// catalog.json describing them, into $KASHI_RECEIPT_PREVIEW_OUT. central's
// settings pages show these images (it cannot run this renderer), so rerun
// it with `make receipt-previews` whenever a theme or layout changes.
func TestGenerateReceiptPreviews(t *testing.T) {
	out := os.Getenv("KASHI_RECEIPT_PREVIEW_OUT")
	if out == "" {
		t.Skip("KASHI_RECEIPT_PREVIEW_OUT not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	base := sampleSocials(printReceiptData{ReceiptWidth: 80, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
	type previewMode struct {
		Mode   string        `json:"mode"`
		File   string        `json:"file"`
		Width  int           `json:"width"`
		Height int           `json:"height"`
		Parts  []receiptMark `json:"parts"`
	}
	type previewLayout struct {
		receiptLayoutInfo
		Preview string        `json:"preview"`
		Modes   []previewMode `json:"modes"`
	}
	type previewTheme struct {
		receiptThemeInfo
		Layouts []previewLayout `json:"layouts"`
	}
	var catalog []previewTheme
	for _, theme := range receiptThemeCatalog() {
		entry := previewTheme{receiptThemeInfo: theme}
		for _, layout := range theme.Layouts {
			item := previewLayout{receiptLayoutInfo: layout}
			for _, mode := range receiptPreviewModes {
				data, marks, err := renderReceiptPreviewMarked(receiptPreviewData(base, mode), theme.ID, layout.ID)
				if err != nil {
					t.Fatalf("%s/%s/%s: %v", theme.ID, layout.ID, mode, err)
				}
				name := fmt.Sprintf("%s-%s.png", theme.ID, layout.ID)
				if mode != receiptModeSale {
					name = fmt.Sprintf("%s-%s-%s.png", theme.ID, layout.ID, mode)
				}
				if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
					t.Fatal(err)
				}
				w, h := pngSize(t, data)
				item.Modes = append(item.Modes, previewMode{Mode: mode, File: name, Width: w, Height: h, Parts: marks})
				if mode == receiptModeSale {
					item.Preview = name
				}
			}
			entry.Layouts = append(entry.Layouts, item)
		}
		catalog = append(catalog, entry)
	}
	defaults := receiptTexts{}
	raw, err := json.MarshalIndent(map[string]any{
		"default_theme": defaultReceiptTheme, "default_layout": defaultReceiptLayout, "themes": catalog,
		// The wording a till prints when central sets none, and the limits it
		// keeps to (receipt_texts.go), so the designer shows and checks the same.
		"texts": map[string]any{
			"defaults": map[string]string{
				"payments_title": defaults.paymentsTitle(), "policy_title": defaults.policyTitle(),
				"referral_title": defaults.referralTitle(), "referral_note": defaultReferralNote,
				"socials_title": defaults.socialsTitle(), "thanks": defaults.thanks(""),
			},
			"sections":         receiptSections,
			"max_length":       receiptTextMaxLen,
			"referral_percent": sampleReferralPercent,
		},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "catalog.json"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pngSize(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestResolveReceiptThemeFallsBack(t *testing.T) {
	cases := []struct{ theme, layout, wantTheme, wantLayout string }{
		{"", "", "classic", "A"},
		{"nonsense", "Z", "classic", "A"},
		{"classic", "C", "classic", "C"},
		{"BOUTIQUE", "b", "boutique", "B"},
		{"boutique", "C", "boutique", "A"}, // boutique has no layout C
		{"supermarket", "", "supermarket", "A"},
	}
	for _, tc := range cases {
		gotTheme, gotLayout := normalizeReceiptLook(tc.theme, tc.layout)
		if gotTheme != tc.wantTheme || gotLayout != tc.wantLayout {
			t.Errorf("normalizeReceiptLook(%q, %q) = %q/%q, want %q/%q", tc.theme, tc.layout, gotTheme, gotLayout, tc.wantTheme, tc.wantLayout)
		}
	}
}

// Every theme and layout must render a sale at both paper widths without
// error, and produce a receipt of sane height.
func TestEveryReceiptLookRenders(t *testing.T) {
	for _, width := range []int64{80, 58} {
		base := printReceiptData{ReceiptWidth: width, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"}
		for _, theme := range receiptThemeCatalog() {
			for _, layout := range theme.Layouts {
				data, err := renderReceiptPreview(sampleReceiptData(base), theme.ID, layout.ID)
				if err != nil {
					t.Fatalf("%dmm %s/%s: %v", width, theme.ID, layout.ID, err)
				}
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dy() < 600 || img.Bounds().Dy() > rasterMaxHeight {
					t.Errorf("%dmm %s/%s: implausible height %d", width, theme.ID, layout.ID, img.Bounds().Dy())
				}
			}
		}
	}
}
