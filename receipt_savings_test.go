package main

import (
	"image"
	"image/color"
	"testing"
)

func TestReceiptSavings(t *testing.T) {
	cases := []struct {
		item    float64
		invoice int64
		want    string
	}{
		{0, 0, ""},
		{0.001, 0, ""},
		{11.2, 0, "You saved $11.20 today"},
		{0, 500, "You saved $5.00 today"},
		{11.2, 500, "You saved $16.20 today"},
	}
	for _, tc := range cases {
		if got := receiptSavings(tc.item, tc.invoice); got != tc.want {
			t.Errorf("receiptSavings(%v, %d) = %q, want %q", tc.item, tc.invoice, got, tc.want)
		}
	}
}

// The total box border is unbroken: every pixel along its top and left edge
// is black, where the dotted box left gaps.
func TestSolidBoxIsUnbroken(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	c := &receiptCanvas{img: img, margin: 10, contentW: 180}
	c.solidBox(10, 60)
	for x := 10; x < 190; x++ {
		if img.RGBAAt(x, 10) != (color.RGBA{0, 0, 0, 255}) || img.RGBAAt(x, 70) != (color.RGBA{0, 0, 0, 255}) {
			t.Fatalf("gap in horizontal border at x=%d", x)
		}
	}
	for y := 10; y <= 70; y++ {
		if img.RGBAAt(10, y) != (color.RGBA{0, 0, 0, 255}) || img.RGBAAt(189, y) != (color.RGBA{0, 0, 0, 255}) {
			t.Fatalf("gap in vertical border at y=%d", y)
		}
	}
}
