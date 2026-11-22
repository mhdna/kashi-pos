package main

// What the receipt renderer needs from the rest of the till: its fonts and
// icons, the line and payment types it prints and a few formatters. Nothing
// here touches the database or the app, so the renderer (the files listed in
// scripts/receipt-render-files.txt) also builds on its own; central copies
// it to draw the Receipt Designer's live receipt (make receipt-renderer).

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
)

//go:embed assets/fonts/Belleza-Regular.ttf
var bellezaFont []byte

//go:embed assets/fonts/CascadiaMono-Regular.ttf
var cascadiaMonoRegular []byte

//go:embed assets/fonts/CascadiaMono-Bold.ttf
var cascadiaMonoBold []byte

//go:embed pdf-icons/facebook.png
var facebookIcon []byte

//go:embed pdf-icons/instagram.png
var instagramIcon []byte

//go:embed pdf-icons/threads.png
var threadsIcon []byte

//go:embed pdf-icons/tiktok.png
var tiktokIcon []byte

type invoiceItem struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Barcode  string  `json:"barcode"`
	Category string  `json:"category"`
	Color    string  `json:"color"`
	Size     string  `json:"size"`
	Price    float64 `json:"price"`
	Discount float64 `json:"discount"`
	Qty      float64 `json:"qty"`
	// A deal line (deals.go): its deal, whether it is free, and the deal's
	// saving in dollars, which the receipt prints instead of working one out
	// from Discount (a bundle's share is not a whole percent).
	Deal         string  `json:"deal,omitempty"`
	Free         bool    `json:"free,omitempty"`
	DealDiscount float64 `json:"dealDiscount,omitempty"`
	// Set by itemsAsPrinted from the receipt design, never sent: the code
	// printed under the name ("" for none), and no before-and-after line.
	PrintCode   string `json:"-"`
	NoPriceNote bool   `json:"-"`
}

func formatInt(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	digits := fmt.Sprintf("%d", value)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}

func formatReceiptPayment(payment receiptPayment, roundingMode string) string {
	amount := payment.AmountCents
	if amount < 0 {
		amount = -amount
	}
	// A whole-major-unit rounding unit greater than 1 identifies a physical
	// cash currency that should be displayed as a rounded integer amount
	// (e.g. LBP); anything else (the default accounting currency, cards)
	// displays as a decimal dollar amount.
	if payment.Currency.CashRoundingUnit > 1 {
		if rounded, err := roundCurrencyUnitsFromCents(amount, payment.Currency.UnitsPerUsdMicros, payment.Currency.CashRoundingUnit, roundingMode); err == nil {
			return fmt.Sprintf("%s %s", formatInt(rounded), payment.Currency.Symbol)
		}
		return "0 " + payment.Currency.Symbol
	}
	return fmt.Sprintf("$%.2f", float64(amount)/100)
}

type socialFooterItem struct {
	platform string
	handle   string
}

func socialFooterItems(platforms []string, handles map[string]string) []socialFooterItem {
	items := make([]socialFooterItem, 0, len(platforms))
	for _, platform := range platforms {
		handle := strings.TrimSpace(handles[platform])
		if handle == "" {
			continue
		}
		items = append(items, socialFooterItem{platform: platform, handle: handle})
	}
	return items
}

func socialPlatformIconBytes(platform string) []byte {
	switch platform {
	case "facebook":
		return facebookIcon
	case "instagram":
		return instagramIcon
	case "threads":
		return threadsIcon
	case "tiktok":
		return tiktokIcon
	default:
		return nil
	}
}

// invoiceDiscount is what a sale gets: the rule's name and the cents it
// takes off. The zero value is none. A referral reward (referrals.go) is
// one too, with its uid in RewardUID.
type invoiceDiscount struct {
	Name      string
	Cents     int64
	RewardUID string
	// A rule that replaces the lines' own discounts is among it
	// (priceInvoiceTotals prices the sale both ways).
	replacesItemDiscounts bool
}

// invoiceDiscountLabel is the receipt row's name: the rule's, or a plain
// one for a rule saved without.
func invoiceDiscountLabel(d invoiceDiscount) string {
	if d.Name == "" {
		return "Invoice discount"
	}
	return d.Name
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// anonymousClientLabel is how the till's screens name a sale with no client.
// Printed receipts keep receiptWalkInLabel instead.
const (
	anonymousClientLabel = "Walk-In"
	receiptWalkInLabel   = "Walk-in"
)

// shiftSpendLine is one expense or loan as the end-of-shift report prints it.
type shiftSpendLine struct {
	Kind         string // "expense" or "loan"
	Label        string
	Amount       int64 // the account's currency, x100, as entered
	CurrencyCode string
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

// Social icons, decoded and scaled once per size: every receipt with socials
// draws them, and decoding and scaling them each time was most of a
// receipt's drawing time.
var (
	socialIconMu    sync.Mutex
	socialIconCache = map[string]*image.RGBA{}
)

// scaledSocialIcon is a platform's icon at size pixels, or nil for a
// platform without one.
func scaledSocialIcon(platform string, size int) (*image.RGBA, error) {
	key := fmt.Sprintf("%s:%d", platform, size)
	socialIconMu.Lock()
	defer socialIconMu.Unlock()
	if icon, ok := socialIconCache[key]; ok {
		return icon, nil
	}
	raw := socialPlatformIconBytes(platform)
	if len(raw) == 0 {
		socialIconCache[key] = nil
		return nil, nil
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode %s icon: %w", platform, err)
	}
	icon := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(icon, icon.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	socialIconCache[key] = icon
	return icon, nil
}
