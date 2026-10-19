package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"
)

// Receipt previews: the real renderer run over a fixed sample sale. The till's
// settings page shows them live (with the branch's own name and phone), and
// the generator in receipt_preview_test.go writes the same images for kashi's
// settings pages, which cannot run this renderer themselves.

// sampleReceiptItems is a mixed basket: long names that wrap, discounts, a
// multi-quantity line, and size and colour for the themes that show them.
func sampleReceiptItems() []invoiceItem {
	return []invoiceItem{
		{Name: "Blue Linen Shirt", Code: "SH-1042", Color: "Navy", Size: "M", Price: 20, Discount: 10, Qty: 2},
		{Name: "Black Slim Fit Stretch Jeans", Code: "JN-220", Color: "Black", Size: "32", Price: 45, Qty: 1},
		{Name: "Cotton Socks", Code: "SK-007", Color: "White", Size: "L", Price: 2, Qty: 5},
		{Name: "Leather Belt Brown", Code: "BL-310", Price: 18, Discount: 20, Qty: 1},
		{Name: "Summer Floral Dress Long Sleeve", Code: "DR-908", Color: "Rose", Size: "S", Price: 60, Qty: 1},
		{Name: "Canvas Tote Bag", Code: "BG-115", Price: 12, Discount: 10, Qty: 3},
		{Name: "Wool Scarf", Code: "SC-042", Color: "Grey", Price: 25, Qty: 1},
		{Name: "Sport Cap", Code: "CP-001", Price: 9, Qty: 2},
	}
}

// sampleReceiptData is a finished sale for the given shop, ready to render.
// base carries the branch's real printing settings (fonts, sizes, width,
// return policy, socials); only the sale itself is invented.
func sampleReceiptData(base printReceiptData) printReceiptData {
	data := base
	data.Items = sampleReceiptItems()
	if data.StoreName == "" {
		data.StoreName = "Your Store"
	}
	if data.Branch == "" {
		data.Branch = "Main Branch"
	}
	if data.BranchPhone == "" && data.Phone == "" {
		data.BranchPhone = "03 123 456"
	}
	data.Salesperson = "Rana"
	data.Customer = ""
	data.ReceiptNo = "PV-10452"
	data.Cutoff = true
	var sub, disc float64
	for _, item := range data.Items {
		gross, discount, _ := itemAmounts(item)
		sub += gross
		disc += discount
	}
	total := int64((sub-disc)*100 + 0.5)
	data.GrandTotalCents = total
	data.RoundingCurrency = receiptCurrency{
		Code: "LBP", Symbol: "L.L.",
		UnitsPerUsdMicros: 90000 * unitsPerUsdMicroScale, CashRoundingUnit: 10000,
	}
	data.RoundingMode = roundingHalfUp
	data.Payments = []receiptPayment{
		{AccountName: "Cash", CurrencyCode: "USD", AmountCents: total + 500, EntryType: "payment"},
		{AccountName: "Cash", CurrencyCode: "USD", AmountCents: -500, EntryType: "change"},
	}
	return data
}

// renderReceiptPreview renders data in the named look as the printer will
// print it: pure black and white, using the same threshold as the raster
// encoder, so thin fonts that would break up on paper break up here too.
func renderReceiptPreview(data printReceiptData, themeID, layoutID string) ([]byte, error) {
	data.Theme, data.Layout = themeID, layoutID
	previousNow := receiptNow
	receiptNow = func() time.Time { return time.Date(2026, 10, 19, 14, 30, 0, 0, time.UTC) }
	defer func() { receiptNow = previousNow }()

	img, err := renderReceiptRaster(data)
	if err != nil {
		return nil, err
	}
	return encodeBilevelPNG(img)
}

func encodeBilevelPNG(img *image.RGBA) ([]byte, error) {
	bounds := img.Bounds()
	out := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.RGBAAt(x, y)
			if pixel.A > 32 && (int(pixel.R)+int(pixel.G)+int(pixel.B))/3 < 176 {
				out.SetColorIndex(x, y, 1)
			}
		}
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("encode receipt preview: %w", err)
	}
	return buf.Bytes(), nil
}
