package main

import (
	"bytes"
	"image/png"
	"testing"
)

// The ribbon is an outline with black letters: a solid black band printed
// grainy on thermal paper. So the band's middle row is white at both inner
// ends, and black ink stays a small share of the picture, for any title.
func TestRibbonIsOutlinedNotSolid(t *testing.T) {
	for _, title := range []string{"BUY 1 GET 1", "STUDENT DEAL", "A", "YOU SAVED"} {
		for _, sub := range []string{"", "with today's deal"} {
			raw, err := renderBannerPreview(receiptBanner{Style: "ribbon", Title: title, Subtitle: sub})
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			black := 0
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
						black++
					}
				}
			}
			if share := float64(black) / float64(b.Dx()*b.Dy()); share > 0.25 {
				t.Errorf("%q/%q: %.0f%% black, the ribbon is solid again", title, sub, share*100)
			}
			// A few pixels in from the left edge at mid height is the band's inside.
			if r, _, _, _ := img.At(b.Min.X+24+30, (b.Min.Y+b.Max.Y)/2).RGBA(); r < 0x8000 {
				t.Errorf("%q/%q: inside of the ribbon is black", title, sub)
			}
		}
	}
}
