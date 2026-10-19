package main

import (
	_ "embed"
	"strings"
)

// A receipt look is a theme (fonts, rules, header, total block, footer) plus
// one of that theme's layouts (how the item lines are arranged). Both are
// stored as short ids in settings (receipt_theme / receipt_layout) and synced
// with kashi. The raster renderer (escpos_raster.go) is the only consumer.
//
// The default is the original look, theme "classic" with layout "A" (the four
// column ITEM / QTY / DISC / TOTAL table). Unknown ids fall back to it so a
// typo or a theme removed later never leaves a till unable to print.

const (
	defaultReceiptTheme  = "classic"
	defaultReceiptLayout = "A"
)

//go:embed assets/fonts/themes/IBMPlexMono-Regular.ttf
var plexMonoRegular []byte

//go:embed assets/fonts/themes/IBMPlexMono-Bold.ttf
var plexMonoBold []byte

//go:embed assets/fonts/themes/CourierPrime-Regular.ttf
var courierPrimeRegular []byte

//go:embed assets/fonts/themes/CourierPrime-Bold.ttf
var courierPrimeBold []byte

//go:embed assets/fonts/themes/CormorantGaramond-Medium.ttf
var cormorantMedium []byte

//go:embed assets/fonts/themes/CormorantGaramond-Bold.ttf
var cormorantBold []byte

//go:embed assets/fonts/themes/Jost-Regular.ttf
var jostRegular []byte

//go:embed assets/fonts/themes/Jost-SemiBold.ttf
var jostSemiBold []byte

//go:embed assets/fonts/themes/Manrope-Medium.ttf
var manropeMedium []byte

//go:embed assets/fonts/themes/Manrope-ExtraBold.ttf
var manropeExtraBold []byte

//go:embed assets/fonts/themes/PlayfairDisplay-ExtraBold.ttf
var playfairExtraBold []byte

//go:embed assets/fonts/themes/Lora-Regular.ttf
var loraRegular []byte

//go:embed assets/fonts/themes/Lora-Bold.ttf
var loraBold []byte

// receiptFontSet names the font files a theme draws with. A nil set means the
// classic selection (Go Mono / Go Bold, or the Belleza / Cascadia choices in
// the branch's font settings).
type receiptFontSet struct {
	Title     []byte // store name and header lines
	TitleBold []byte
	Body      []byte // everyday text
	BodyBold  []byte // headings, totals
	Name      []byte // item names; falls back to BodyBold
}

// ruleKind is how a horizontal divider is drawn.
type ruleKind int

const (
	ruleDotted ruleKind = iota
	ruleDashed
	ruleSolid
	ruleDouble
	ruleOrnament
)

// totalKind is how the TOTAL line is presented.
type totalKind int

const (
	totalBox   totalKind = iota // dotted box around TOTAL (classic)
	totalBand                   // white on black bar
	totalRuled                  // large text between two rules
)

type receiptStyle struct {
	Fonts *receiptFontSet
	// Scales stretch point sizes for faces that read small (serifs) or large.
	TitleScale, BodyScale float64

	UpperStore   bool // store and branch name in capitals
	Rule         ruleKind
	MetaRows     bool // Customer / Salesperson / Date as label-left value-right rows
	MetaCentered bool
	Total        totalKind
	Thanks       string
	ItemCount    bool // "N items" line above the totals
	Savings      bool // "You saved $X" line when anything was discounted
}

// receiptLayoutSpec is one arrangement of the item lines within a theme.
// draw prints the column header (if any) and every item, and returns the new
// y plus the undiscounted subtotal and the total discount, which the totals
// block needs.
type receiptLayoutSpec struct {
	ID          string
	Name        string
	Description string
	draw        func(c *receiptCanvas, y int) (int, float64, float64)
}

type receiptTheme struct {
	ID          string
	Name        string
	Description string
	BestFor     string
	Style       receiptStyle
	Layouts     []receiptLayoutSpec
}

// receiptThemes is the catalog, in the order the settings pages list it. The
// first entry is the default.
func receiptThemes() []receiptTheme {
	return []receiptTheme{
		{
			ID: "classic", Name: "Classic",
			Description: "The original look: dotted dividers, boxed total, your own font choices.",
			BestFor:     "Any shop",
			Style: receiptStyle{
				TitleScale: 1, BodyScale: 1,
				Rule: ruleDotted, Total: totalBox,
				Thanks: "Thank you for shopping with us!",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Columns", "Item, quantity, discount and total on one line each.", drawClassicColumns},
				{"B", "Price columns", "Item, quantity, unit price and total; discounts on their own line.", drawClassicPriceColumns},
				{"C", "Detailed", "Name and total, then code and quantity lines. The longest.", drawClassicDetailed},
			},
		},
		{
			ID: "supermarket", Name: "Supermarket",
			Description: "Dense monospaced register tape in capitals, with item count and savings.",
			BestFor:     "Supermarkets, groceries, pharmacies: many items per sale",
			Style: receiptStyle{
				Fonts:      &receiptFontSet{Title: plexMonoBold, TitleBold: plexMonoBold, Body: plexMonoRegular, BodyBold: plexMonoBold, Name: plexMonoRegular},
				TitleScale: 0.82, BodyScale: 0.86,
				UpperStore: true, Rule: ruleDashed, Total: totalBand, ItemCount: true, Savings: true,
				Thanks: "THANK YOU, COME AGAIN",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Register", "Name and amount; quantity and savings only when they matter.", drawSupermarketRegister},
				{"B", "Quantity first", "A quantity column on the left, like a till tape.", drawSupermarketQuantity},
				{"C", "With codes", "Each item followed by its code and unit price.", drawSupermarketCodes},
			},
		},
		{
			ID: "boutique", Name: "Boutique",
			Description: "Elegant serif with size and colour shown, centered and spacious.",
			BestFor:     "Clothing, shoes, jewellery, gifts",
			Style: receiptStyle{
				Fonts:      &receiptFontSet{Title: cormorantBold, TitleBold: cormorantBold, Body: jostRegular, BodyBold: jostSemiBold, Name: cormorantBold},
				TitleScale: 1.3, BodyScale: 0.95,
				Rule: ruleOrnament, MetaRows: true, Total: totalRuled,
				Thanks: "Thank you for shopping with us",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Centered", "Each item centered with its size, colour and price beneath.", drawBoutiqueCentered},
				{"B", "Ledger", "Name and total on the left and right, details beneath.", drawBoutiqueLedger},
			},
		},
		{
			ID: "modern", Name: "Modern",
			Description: "Clean geometric sans with a solid total bar.",
			BestFor:     "Cafés, electronics, cosmetics, any contemporary shop",
			Style: receiptStyle{
				Fonts:      &receiptFontSet{Title: manropeExtraBold, TitleBold: manropeExtraBold, Body: manropeMedium, BodyBold: manropeExtraBold, Name: manropeExtraBold},
				TitleScale: 1.05, BodyScale: 0.98,
				Rule: ruleSolid, MetaRows: true, Total: totalBand,
				Thanks: "Thanks for your visit!",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Clean", "Name and total, quantity and savings on a quiet second line.", drawModernClean},
				{"B", "Table", "A table with a dark header bar.", drawModernTable},
			},
		},
		{
			ID: "bistro", Name: "Bistro",
			Description: "Menu style with dot leaders and a double-ruled total.",
			BestFor:     "Restaurants, bakeries, delis, florists",
			Style: receiptStyle{
				Fonts:      &receiptFontSet{Title: playfairExtraBold, TitleBold: playfairExtraBold, Body: loraRegular, BodyBold: loraBold, Name: loraBold},
				TitleScale: 1, BodyScale: 0.96,
				Rule: ruleDouble, MetaCentered: true, Total: totalRuled,
				Thanks: "Thank you, see you again!",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Menu", "Name, dot leader, price; quantity beneath when more than one.", drawBistroMenu},
				{"B", "Quantity first", "2 x Name ...... price.", drawBistroQuantity},
			},
		},
		{
			ID: "typewriter", Name: "Typewriter",
			Description: "Courier tape in capitals with dashed dividers.",
			BestFor:     "Bookshops, hardware, vintage and craft shops",
			Style: receiptStyle{
				Fonts:      &receiptFontSet{Title: courierPrimeBold, TitleBold: courierPrimeBold, Body: courierPrimeRegular, BodyBold: courierPrimeBold, Name: courierPrimeBold},
				TitleScale: 1, BodyScale: 1,
				UpperStore: true, Rule: ruleDashed, Total: totalRuled,
				Thanks: "*** THANK YOU ***",
			},
			Layouts: []receiptLayoutSpec{
				{"A", "Tape", "Fixed columns: quantity, item, price, total.", drawTypewriterTape},
				{"B", "Itemized", "Item, then quantity at price, then any discount.", drawTypewriterItemized},
			},
		},
	}
}

// resolveReceiptTheme returns the named theme and layout, falling back to
// the default theme, and to a theme's first layout when the layout id isn't
// one of its own.
func resolveReceiptTheme(themeID, layoutID string) (receiptTheme, receiptLayoutSpec) {
	themes := receiptThemes()
	theme := themes[0]
	for _, candidate := range themes {
		if strings.EqualFold(candidate.ID, strings.TrimSpace(themeID)) {
			theme = candidate
			break
		}
	}
	layout := theme.Layouts[0]
	for _, candidate := range theme.Layouts {
		if strings.EqualFold(candidate.ID, strings.TrimSpace(layoutID)) {
			layout = candidate
			break
		}
	}
	return theme, layout
}

// normalizeReceiptLook returns the canonical ids for a theme/layout pair, so
// what is stored and synced is always a pair the renderer will honour.
func normalizeReceiptLook(themeID, layoutID string) (string, string) {
	theme, layout := resolveReceiptTheme(themeID, layoutID)
	return theme.ID, layout.ID
}

// receiptThemeInfo is the catalog as JSON for the settings pages and for the
// preview generator that feeds kashi.
type receiptThemeInfo struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	BestFor     string              `json:"best_for"`
	Layouts     []receiptLayoutInfo `json:"layouts"`
}

type receiptLayoutInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func receiptThemeCatalog() []receiptThemeInfo {
	themes := receiptThemes()
	out := make([]receiptThemeInfo, 0, len(themes))
	for _, theme := range themes {
		info := receiptThemeInfo{ID: theme.ID, Name: theme.Name, Description: theme.Description, BestFor: theme.BestFor}
		for _, layout := range theme.Layouts {
			info.Layouts = append(info.Layouts, receiptLayoutInfo{ID: layout.ID, Name: layout.Name, Description: layout.Description})
		}
		out = append(out, info)
	}
	return out
}
