package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestBuildESCPOSRasterReceiptUsesSafeBands(t *testing.T) {
	payload, err := buildESCPOSRasterReceipt(rasterTestReceipt(80))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte("%PDF")) {
		t.Fatal("raster payload contains PDF data")
	}
	if err := validateRasterPayload(payload, raster80mmWidth); err != nil {
		t.Fatalf("invalid raster payload: %v", err)
	}
	img := decodeRasterTestImage(t, payload, raster80mmWidth)
	if img.Bounds().Dy() < 500 {
		t.Fatalf("receipt raster is unexpectedly short: %d", img.Bounds().Dy())
	}
	if !bytes.HasSuffix(payload, []byte{gs, cutMode, 0}) {
		t.Fatal("cutoff-enabled receipt does not end with the full-cut command")
	}
	blackPixels := 0
	firstBlackRow := img.Bounds().Dy()
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.GrayAt(x, y).Y == 0 {
				blackPixels++
				if y < firstBlackRow {
					firstBlackRow = y
				}
			}
		}
	}
	if firstBlackRow != 0 {
		t.Fatalf("receipt has top padding: first content row is %d", firstBlackRow)
	}
	if blackPixels < 1000 {
		t.Fatalf("receipt raster contains too little printable content: %d pixels", blackPixels)
	}
	if output := os.Getenv("KASHI_RASTER_TEST_OUTPUT"); output != "" {
		file, err := os.Create(output)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildESCPOSRasterReceiptOmitsCutoffWhenDisabled(t *testing.T) {
	data := rasterTestReceipt(80)
	data.Cutoff = false
	payload, err := buildESCPOSRasterReceipt(data)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte{gs, cutMode}) {
		t.Fatal("cutoff-disabled receipt contains a cut command")
	}
}

func TestBuildESCPOSRasterReceiptUses384DotsFor58mm(t *testing.T) {
	payload, err := buildESCPOSRasterReceipt(rasterTestReceipt(58))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRasterPayload(payload, raster58mmWidth); err != nil {
		t.Fatalf("invalid 58mm raster payload: %v", err)
	}
}

func TestValidateRasterPayloadRejectsPDF(t *testing.T) {
	if err := validateRasterPayload([]byte("%PDF-1.4"), raster80mmWidth); err == nil {
		t.Fatal("expected PDF payload to be rejected")
	}
}

func rasterTestReceipt(width int64) printReceiptData {
	return printReceiptData{
		Items: []invoiceItem{
			{Name: "Classic T-Shirt", Qty: 2, Price: 18},
			{Name: "Canvas Sneakers", Qty: 1, Price: 42.5, Discount: 10},
		},
		StoreName:       "Kashi Market",
		Branch:          "Downtown Branch",
		BranchPhone:     "+961 1 234 567",
		Description:     "Everyday essentials",
		RegisterName:    "POS-04",
		Salesperson:     "Sarah Demo",
		Customer:        "Walk-in",
		ReturnPolicy:    "Exchanges accepted within 3 days.",
		Website:         "kashi.example",
		SocialPlatforms: []string{"facebook", "tiktok", "instagram", "threads", "youtube"},
		SocialHandles: map[string]string{
			"facebook": "@kashi", "tiktok": "@kashi", "instagram": "@kashi",
			"threads": "@kashi", "youtube": "Kashi Market",
		},
		Font: "Belleza", BodyFont: "Belleza", TitleSize: 21, BodySize: 11,
		ReceiptNo: "INV-TEST-0001", GrandTotalCents: 7900,
		Payments:         []receiptPayment{{AccountName: "Cash", CurrencyCode: "USD", AmountCents: 7900}},
		RoundingCurrency: testLBPReceiptCurrency, RoundingMode: "nearest", ReceiptWidth: width, Cutoff: true,
	}
}

func decodeRasterTestImage(t *testing.T, payload []byte, width int) *image.Gray {
	t.Helper()
	offset := 8
	rows := make([][]byte, 0)
	for offset+2 <= len(payload) && !(payload[offset] == esc && payload[offset+1] == '2') {
		rowBytes := int(payload[offset+4]) | int(payload[offset+5])<<8
		height := int(payload[offset+6]) | int(payload[offset+7])<<8
		offset += 8
		for row := 0; row < height; row++ {
			rows = append(rows, append([]byte(nil), payload[offset:offset+rowBytes]...))
			offset += rowBytes
		}
		offset++
	}
	img := image.NewGray(image.Rect(0, 0, width, len(rows)))
	for y, row := range rows {
		for x := 0; x < width; x++ {
			shade := uint8(255)
			if row[x/8]&(1<<uint(7-x%8)) != 0 {
				shade = 0
			}
			img.SetGray(x, y, color.Gray{Y: shade})
		}
	}
	return img
}
