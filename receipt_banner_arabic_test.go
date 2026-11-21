package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func bannerImage(t *testing.T, b receiptBanner) image.Image {
	t.Helper()
	raw, err := renderBannerPreview(b)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func inkIn(img image.Image, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if rr, _, _, _ := img.At(x, y).RGBA(); rr < 0x8000 {
				n++
			}
		}
	}
	return n
}

// An Arabic line under the title is drawn (the style's fonts have no Arabic,
// so it used to vanish), and it is not cut off at the bottom of the band.
func TestBannerArabicSubtitleIsDrawn(t *testing.T) {
	with := bannerImage(t, receiptBanner{Style: "ribbon", Title: "50$ CASHBACK", Subtitle: "بمناسبة عيد الاستقلال"})
	without := bannerImage(t, receiptBanner{Style: "ribbon", Title: "50$ CASHBACK"})
	b := with.Bounds()
	if with.Bounds().Dy() <= without.Bounds().Dy() {
		t.Fatalf("no room made for the Arabic line: %d vs %d", with.Bounds().Dy(), without.Bounds().Dy())
	}
	// The middle of the band's lower half holds the Arabic letters.
	if inkIn(with, image.Rect(150, b.Dy()*55/100, 420, b.Dy()*85/100)) < 150 {
		t.Fatal("the Arabic line is missing")
	}
}

// The Lebanese flag emoji becomes a drawn flag each side of the words, and is
// never printed as text.
func TestBannerLebanonFlagDrawnBothSides(t *testing.T) {
	plain := bannerImage(t, receiptBanner{Style: "ribbon", Title: "X", Subtitle: "عيد الاستقلال"})
	flag := bannerImage(t, receiptBanner{Style: "ribbon", Title: "X", Subtitle: "عيد الاستقلال " + lebanonFlag})
	b := flag.Bounds()
	left := image.Rect(0, b.Dy()*50/100, b.Dx()/2-60, b.Dy()*90/100)
	right := image.Rect(b.Dx()/2+60, b.Dy()*50/100, b.Dx(), b.Dy()*90/100)
	if inkIn(flag, left) <= inkIn(plain, left)+80 || inkIn(flag, right) <= inkIn(plain, right)+80 {
		t.Fatalf("flags missing: left %d/%d right %d/%d", inkIn(flag, left), inkIn(plain, left), inkIn(flag, right), inkIn(plain, right))
	}
}

// A Latin subtitle keeps its old height (24 under the title), and an Arabic
// one adds room for its taller letters.
func TestBannerLatinKeepsItsHeight(t *testing.T) {
	latin := bannerImage(t, receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", Subtitle: "Today only"})
	arabic := bannerImage(t, receiptBanner{Style: "ribbon", Title: "BUY 1 GET 1", Subtitle: "اليوم فقط"})
	if arabic.Bounds().Dy()-latin.Bounds().Dy() != 14 {
		t.Fatalf("heights: latin %d, arabic %d", latin.Bounds().Dy(), arabic.Bounds().Dy())
	}
}
