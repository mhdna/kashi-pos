package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"strings"
	"time"
)

// The renderer as central uses it (its copy of these files, package
// receiptrender): the Receipt Designer draws the branch's real receipt with
// the design being edited, from this same code, so what it shows is what
// the till prints.

// DesignerShop is the branch's own data the receipt prints.
type DesignerShop struct {
	StoreName       string            `json:"store_name"`
	Branch          string            `json:"branch"`
	Phone           string            `json:"phone"`
	Website         string            `json:"website"`
	ReturnPolicy    string            `json:"return_policy"`
	SocialPlatforms []string          `json:"social_platforms"`
	SocialHandles   map[string]string `json:"social_handles"`
	Font            string            `json:"font"`
	BodyFont        string            `json:"body_font"`
	TitleSize       float64           `json:"title_size"`
	BodySize        float64           `json:"body_size"`
	ReceiptWidth    int64             `json:"receipt_width"`
}

// DesignerInput is one preview: a look, a mode, the design (receipt_texts
// JSON) and the shop. ReferralPercent is the reward the referral mode shows.
type DesignerInput struct {
	Theme           string
	Layout          string
	Mode            string
	Texts           string
	Shop            DesignerShop
	ReferralPercent int64
}

// DesignerMark is where one part of the receipt is, for the designer to open.
type DesignerMark struct {
	Group  string `json:"group"`
	Top    int    `json:"top"`
	Bottom int    `json:"bottom"`
}

// DesignerPreview is the drawn receipt (PNG, as the printer prints it) and
// where its parts are, merged per part.
type DesignerPreview struct {
	PNG    []byte         `json:"-"`
	Width  int            `json:"width"`
	Height int            `json:"height"`
	Marks  []DesignerMark `json:"marks"`
}

// RenderDesignerPreview draws the sample receipt for a branch with a design.
func RenderDesignerPreview(in DesignerInput) (DesignerPreview, error) {
	base := printReceiptData{
		StoreName: in.Shop.StoreName, Branch: in.Shop.Branch, BranchPhone: in.Shop.Phone, Website: in.Shop.Website,
		ReturnPolicy: in.Shop.ReturnPolicy, SocialPlatforms: in.Shop.SocialPlatforms, SocialHandles: in.Shop.SocialHandles,
		Font: in.Shop.Font, BodyFont: in.Shop.BodyFont, TitleSize: in.Shop.TitleSize, BodySize: in.Shop.BodySize,
		ReceiptWidth: in.Shop.ReceiptWidth,
	}
	if base.ReceiptWidth == 0 {
		base.ReceiptWidth = 80
	}
	if base.TitleSize == 0 {
		base.TitleSize = 21
	}
	if base.BodySize == 0 {
		base.BodySize = 11
	}
	if base.BodyFont == "" {
		base.BodyFont = "Printer Font A"
	}
	if base.Font == "" {
		base.Font = "Belleza"
	}
	base = sampleSocials(base)
	base.Texts = normalizeReceiptTexts(parseReceiptTexts(in.Texts))
	data := receiptPreviewData(base, in.Mode)
	if data.ReferralCode != "" {
		percent := in.ReferralPercent
		if percent <= 0 {
			percent = sampleReferralPercent
		}
		if data.Texts.hidden(receiptSectionReferral) {
			data.ReferralCode, data.ReferralNote = "", ""
		} else {
			data.ReferralNote = data.Texts.referralNote(percent)
		}
	}
	png, marks, err := renderDesignerPNG(data, in.Theme, in.Layout)
	if err != nil {
		return DesignerPreview{}, err
	}
	w, h := pngDims(png)
	return DesignerPreview{PNG: png, Width: w, Height: h, Marks: designerMarks(marks)}, nil
}

// designerMarks is one band per part: from its first line to its last.
func designerMarks(marks []receiptMark) []DesignerMark {
	var out []DesignerMark
	index := map[string]int{}
	for _, m := range marks {
		if m.Kind != "group" {
			continue
		}
		if i, ok := index[m.Part]; ok {
			out[i].Top = minInt(out[i].Top, m.Top)
			out[i].Bottom = maxInt(out[i].Bottom, m.Bottom)
			continue
		}
		index[m.Part] = len(out)
		out = append(out, DesignerMark{Group: m.Part, Top: m.Top, Bottom: m.Bottom})
	}
	return out
}

// DesignerCatalog is everything the designer offers: looks, modes, the parts
// and their elements with what each can change, and the date formats.
func DesignerCatalog() map[string]any {
	type dateFormat struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	formats := make([]dateFormat, 0, len(receiptDateFormats))
	for _, f := range receiptDateFormats {
		formats = append(formats, dateFormat{f.Key, f.Label})
	}
	return map[string]any{
		"themes":         receiptThemeCatalog(),
		"default_theme":  defaultReceiptTheme,
		"default_layout": defaultReceiptLayout,
		"modes":          receiptPreviewModes,
		"groups":         receiptGroups,
		"elements":       receiptElements,
		"date_formats":   formats,
		"can": map[string]int{
			"text": canText, "case": canCase, "size": canSize, "bold": canBold, "align": canAlign, "hide": canHide,
		},
		"size_min":   receiptElementMinSize,
		"size_max":   receiptElementMaxSize,
		"max_length": receiptTextMaxLen,
	}
}

// NormalizeReceiptTextsJSON is a stored receipt_texts value: cleaned up as the
// till keeps it, "" for every default or anything unreadable.
func NormalizeReceiptTextsJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var t receiptTexts
	if json.Unmarshal([]byte(raw), &t) != nil {
		return ""
	}
	b, err := json.Marshal(normalizeReceiptTexts(t))
	if err != nil || string(b) == "{}" {
		return ""
	}
	return string(b)
}

func pngDims(data []byte) (int, int) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// renderDesignerPNG draws a preview with fast compression: it's drawn again
// on every change, and the best compression was a third of its time.
func renderDesignerPNG(data printReceiptData, theme, layout string) ([]byte, []receiptMark, error) {
	data.Theme, data.Layout = theme, layout
	// The sample's date, given with the receipt: central draws previews for
	// several people at once, so nothing global is changed.
	data.Now = time.Date(2026, 10, 19, 14, 30, 0, 0, time.UTC)
	img, marks, err := renderReceiptRasterMarked(data)
	if err != nil {
		return nil, nil, err
	}
	out, err := encodeBilevelPNGLevel(img, png.BestSpeed)
	return out, marks, err
}
