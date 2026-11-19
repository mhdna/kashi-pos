package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Receipt banners (receiptTexts.Banners), set from central's Receipt
// Designer: a band of display lettering, drawn by the till in black and
// white so it prints crisp on thermal paper. Each has a style, its words,
// when it prints (always, on a buy X get Y sale, with a deal on the invoice,
// or with one deal by name) and where (under the shop's name, or at the end).

type receiptBanner struct {
	// Style is one of receiptBannerStyles.
	Style    string `json:"style"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	// When: "" always, "item_deal" a sale with a deal on its items (buy 2
	// get 1), "deal" a sale with a deal on the invoice, "deal:<name>" a sale
	// with that deal, on the invoice or its items, "item_discount" a sale
	// with an item on sale (its own discount), "referral" a customer with a
	// referral code to share. Any other is left out. Title and Subtitle may
	// hold {deal}: the deal's name (the title's in capitals).
	When string `json:"when,omitempty"`
	// Place: "" under the shop's name, "bottom" at the end of the receipt.
	Place string `json:"place,omitempty"`
	// Font, when set, is the title's font (receiptFonts) instead of the
	// style's own.
	Font string `json:"font,omitempty"`
}

// titleFont is the banner's own font, else the style's.
func (b receiptBanner) titleFont(style string) string {
	if b.Font != "" {
		return b.Font
	}
	return style
}

// receiptBannerStyleInfo describes a style for the designer.
type receiptBannerStyleInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var receiptBannerStyles = []receiptBannerStyleInfo{
	{"ribbon", "Ribbon"},
	{"burst", "Burst"},
	{"stamp", "Stamp"},
	{"frame", "Frame"},
	{"ornament", "Ornament"},
	{"stars", "Stars"},
}

// receiptBannerPresets are ready-made banners the designer offers.
var receiptBannerPresets = []receiptBanner{
	{Style: "ribbon", Title: "BUY 1 GET 1", Subtitle: "FREE", When: "item_deal"},
	{Style: "burst", Title: "BUY 2 GET 1", Subtitle: "The third one is on us", When: "item_deal"},
	{Style: "ribbon", Title: "STUDENT DEAL", When: "deal"},
	{Style: "frame", Title: "SALE", Subtitle: "Up to 50% off"},
	{Style: "ornament", Title: "Thank You", Subtitle: "See you again soon", Place: "bottom"},
	{Style: "stars", Title: "NEW SEASON", Subtitle: "Fresh arrivals in store"},
	{Style: "frame", Title: "VIP", Subtitle: "Members save more"},
	{Style: "burst", Title: "WEEKEND DEAL", Subtitle: "Saturday and Sunday only"},
	{Style: "ribbon", Title: "YOU SAVED", Subtitle: "with today's deal", When: "deal"},
	{Style: "stars", Title: "GIFT", Subtitle: "Wrapped with love"},
}

const (
	// A branch can keep a banner per deal; one prints (chosenBanner).
	receiptBannerMax         = 12
	receiptBannerTitleMax    = 40
	receiptBannerSubtitleMax = 80
)

func validBannerStyle(s string) bool {
	for _, st := range receiptBannerStyles {
		if st.Key == s {
			return true
		}
	}
	return false
}

func clipRunes(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(s) > n {
		s = string([]rune(s)[:n])
	}
	return s
}

// normalizeReceiptBanners keeps up to receiptBannerMax banners with a known
// style and a title, their words cut to size.
func normalizeReceiptBanners(in []receiptBanner) []receiptBanner {
	var out []receiptBanner
	for _, b := range in {
		if len(out) == receiptBannerMax {
			break
		}
		n := receiptBanner{Style: b.Style, Title: clipRunes(b.Title, receiptBannerTitleMax), Subtitle: clipRunes(b.Subtitle, receiptBannerSubtitleMax)}
		if !validBannerStyle(n.Style) || n.Title == "" {
			continue
		}
		switch {
		case b.When == "" || b.When == "item_deal" || b.When == "deal" || b.When == "item_discount" || b.When == "referral":
			n.When = b.When
		case strings.HasPrefix(b.When, "deal:") && strings.TrimSpace(strings.TrimPrefix(b.When, "deal:")) != "":
			n.When = "deal:" + clipRunes(strings.TrimPrefix(b.When, "deal:"), 120)
		default:
			// A condition this till doesn't know (from a newer central) leaves
			// the banner out, rather than printing it on every receipt.
			continue
		}
		if b.Place == "bottom" {
			n.Place = "bottom"
		}
		n.Font = validReceiptFont(b.Font)
		out = append(out, n)
	}
	return out
}

// bannerApplies says whether a banner prints on this receipt.
func (c *receiptCanvas) bannerApplies(b receiptBanner) bool {
	_, ok := c.bannerRank(b)
	return ok
}

// invoiceDealNames are the deals the invoice took off, biggest first: one, or
// several joined with " + " where they added up (invoiceDiscountsStacked).
func (c *receiptCanvas) invoiceDealNames() []string {
	if c.data.InvoiceDiscount.Cents <= 0 || c.data.InvoiceDiscount.Name == "" {
		return nil
	}
	return strings.Split(c.data.InvoiceDiscount.Name, " + ")
}

// bannerRank says whether a banner prints on this receipt and how strongly it
// fits (lower is better): a banner for one deal first, the bigger the deal
// the better (its place in the invoice's deal names), then one for a deal on
// the items, then buy X get Y, then any invoice deal, then always.
func (c *receiptCanvas) bannerRank(b receiptBanner) (int, bool) {
	itemDeal := func(name string) bool {
		for _, item := range c.data.Items {
			if item.Deal != "" && (name == "" || item.Deal == name) {
				return true
			}
		}
		return false
	}
	switch {
	case b.When == "":
		return 300, true
	case b.When == "item_deal":
		return 100, itemDeal("")
	case b.When == "deal":
		return 200, c.data.InvoiceDiscount.Cents > 0
	case b.When == "referral":
		// The customer has a referral code to share (printed at the end).
		return 275, c.data.ReferralCode != ""
	case b.When == "item_discount":
		// An item on sale: a line with its own discount (a deal's lines are
		// the deal banners').
		for _, item := range c.data.Items {
			if item.Discount > 0 && item.Deal == "" {
				return 250, true
			}
		}
		return 0, false
	case strings.HasPrefix(b.When, "deal:"):
		name := strings.TrimPrefix(b.When, "deal:")
		if i := slices.Index(c.invoiceDealNames(), name); i >= 0 {
			return i, true
		}
		return 50, itemDeal(name)
	}
	return 0, false
}

// bannerDealName is the deal a banner is about: its own deal, the deal on
// the items, or the biggest one on the invoice; "" for none.
func (c *receiptCanvas) bannerDealName(b receiptBanner) string {
	if name, ok := strings.CutPrefix(b.When, "deal:"); ok {
		return name
	}
	for _, item := range c.data.Items {
		if item.Deal != "" && (b.When == "item_deal" || b.When == "") {
			return item.Deal
		}
	}
	if names := c.invoiceDealNames(); len(names) > 0 {
		return names[0]
	}
	return ""
}

// buyGetPattern finds "buy 2 get 1" in a deal's name.
var buyGetPattern = regexp.MustCompile(`(?i)\bbuy\s*(\d+)\s*get\s*(\d+)`)

// bannerDealTitle is a deal's name as a banner's title: in capitals, and
// just "BUY 2 GET 1" for a buy X get Y deal, whatever else its name says
// ("Buy 2 get 1 free (otherwise 30%)").
func bannerDealTitle(name string) string {
	if m := buyGetPattern.FindStringSubmatch(name); m != nil {
		return "BUY " + m[1] + " GET " + m[2]
	}
	return strings.ToUpper(name)
}

// withDealName fills {deal} in a banner's words: the title in capitals.
func (c *receiptCanvas) withDealName(b receiptBanner) receiptBanner {
	if !strings.Contains(b.Title+b.Subtitle, "{deal}") {
		return b
	}
	name := c.bannerDealName(b)
	b.Title = strings.TrimSpace(strings.ReplaceAll(b.Title, "{deal}", bannerDealTitle(name)))
	b.Subtitle = strings.TrimSpace(strings.ReplaceAll(b.Subtitle, "{deal}", name))
	return b
}

// chosenBanner is the one banner this receipt prints, -1 for none: the best
// fitting (bannerRank), the first of equals. Never two on one receipt.
func (c *receiptCanvas) chosenBanner() int {
	best, bestRank := -1, 0
	for i, b := range c.data.Texts.Banners {
		if rank, ok := c.bannerRank(b); ok && (best < 0 || rank < bestRank) {
			best, bestRank = i, rank
		}
	}
	return best
}

// drawBanners draws the receipt's one banner (chosenBanner) when it goes in
// this place ("" top, "bottom").
func (c *receiptCanvas) drawBanners(y int, place string) int {
	i := c.chosenBanner()
	if i < 0 || c.data.Texts.Banners[i].Place != place {
		return y
	}
	top := y
	y = c.drawBanner(y, c.withDealName(c.data.Texts.Banners[i])) + c.gap(12)
	c.markGroup(fmt.Sprintf("banner-%d", i), top, y)
	return y
}

// Banner fonts: parsed once, shared.
var (
	bannerFontMu sync.Mutex
	bannerFonts  = map[string]*opentype.Font{}
)

func bannerFontData(name string) []byte {
	if f, ok := receiptFontByKey(name); ok {
		return f.Bold
	}
	switch name {
	case "jost":
		return jostSemiBold
	case "jost-regular":
		return jostRegular
	case "manrope":
		return manropeExtraBold
	case "playfair":
		return playfairExtraBold
	case "cormorant":
		return cormorantBold
	case "lora":
		return loraBold
	}
	return jostSemiBold
}

func (c *receiptCanvas) bannerFace(name string, px float64) (font.Face, error) {
	key := fmt.Sprintf("banner:%s:%.1f", name, px)
	if f := c.faces[key]; f != nil {
		return f, nil
	}
	bannerFontMu.Lock()
	parsed := bannerFonts[name]
	if parsed == nil {
		var err error
		parsed, err = opentype.Parse(bannerFontData(name))
		if err != nil {
			bannerFontMu.Unlock()
			return nil, fmt.Errorf("parse banner font: %w", err)
		}
		bannerFonts[name] = parsed
	}
	bannerFontMu.Unlock()
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	c.faces[key] = face
	return face, nil
}

// fitFace is the biggest face, up to px, that fits text in width.
func (c *receiptCanvas) fitFace(name, text string, px float64, width int) (font.Face, error) {
	for ; px > 14; px -= 2 {
		face, err := c.bannerFace(name, px)
		if err != nil {
			return nil, err
		}
		if font.MeasureString(face, text).Ceil() <= width {
			return face, nil
		}
	}
	return c.bannerFace(name, 14)
}

// bannerText draws text centred between left and right with its top at y,
// in ink, returning the y under it.
func (c *receiptCanvas) bannerText(face font.Face, text string, left, right, y int, ink image.Image) int {
	m := face.Metrics()
	w := font.MeasureString(face, text).Ceil()
	x := left + (right-left-w)/2
	d := font.Drawer{Dst: c.img, Src: ink, Face: face, Dot: fixed.P(x, y+m.Ascent.Ceil())}
	d.DrawString(text)
	if receiptTextDrawn != nil {
		receiptTextDrawn(text, y)
	}
	return y + m.Ascent.Ceil() + m.Descent.Ceil()
}

// fillPolygon fills the polygon pts (even-odd) in ink.
func (c *receiptCanvas) fillPolygon(pts []image.Point, ink color.Color) {
	if len(pts) < 3 {
		return
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts {
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	b := c.img.Bounds()
	for y := max(minY, b.Min.Y); y <= min(maxY, b.Max.Y-1); y++ {
		fy := float64(y) + 0.5
		var xs []float64
		for i := range pts {
			a, z := pts[i], pts[(i+1)%len(pts)]
			if (float64(a.Y) <= fy) == (float64(z.Y) <= fy) {
				continue
			}
			xs = append(xs, float64(a.X)+(fy-float64(a.Y))*float64(z.X-a.X)/float64(z.Y-a.Y))
		}
		for i := 0; i+1 < len(xs); i++ {
			for j := i + 1; j < len(xs); j++ {
				if xs[j] < xs[i] {
					xs[i], xs[j] = xs[j], xs[i]
				}
			}
		}
		for i := 0; i+1 < len(xs); i += 2 {
			x0, x1 := int(math.Ceil(xs[i]-0.5)), int(math.Floor(xs[i+1]-0.5))
			for x := max(x0, b.Min.X); x <= min(x1, b.Max.X-1); x++ {
				c.img.Set(x, y, ink)
			}
		}
	}
}

// star is a five-point star's outline around cx, cy.
func star(cx, cy, r int) []image.Point {
	pts := make([]image.Point, 0, 10)
	for i := 0; i < 10; i++ {
		rr := float64(r)
		if i%2 == 1 {
			rr *= 0.45
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		pts = append(pts, image.Point{cx + int(math.Round(rr*math.Cos(a))), cy + int(math.Round(rr*math.Sin(a)))})
	}
	return pts
}

func diamond(cx, cy, r int) []image.Point {
	return []image.Point{{cx, cy - r}, {cx + r, cy}, {cx, cy + r}, {cx - r, cy}}
}

// drawBanner draws one banner from y and returns the y under it.
func (c *receiptCanvas) drawBanner(y int, b receiptBanner) int {
	left, right := c.margin, c.margin+c.contentW
	white := image.NewUniform(color.White)
	black := image.NewUniform(color.Black)
	sub := func(y int, name string, px float64, ink image.Image, l, r int) int {
		if b.Subtitle == "" {
			return y
		}
		face, err := c.fitFace(name, b.Subtitle, px, r-l-16)
		if err != nil {
			return y
		}
		return c.bannerText(face, b.Subtitle, l, r, y, ink)
	}
	switch b.Style {
	case "ribbon":
		// A black band with notched ends, white letters. Every ribbon is as
		// tall as its title at full size: a longer title that has to shrink
		// to fit ("STUDENT DEAL" beside "BUY 1 GET 1") is centred in the same
		// band, so two receipts' ribbons match.
		face, _ := c.fitFace(b.titleFont("jost"), b.Title, 50, c.contentW-90)
		full := face
		if f, err := c.bannerFace(b.titleFont("jost"), 50); err == nil {
			full = f
		}
		fm, m := full.Metrics(), face.Metrics()
		titleH := fm.Ascent.Ceil() + fm.Descent.Ceil()
		shift := (titleH - (m.Ascent.Ceil() + m.Descent.Ceil())) / 2
		subH := 0
		if b.Subtitle != "" {
			subH = 24
		}
		h := titleH + 14 + subH
		top, bot := y, y+h
		notch := 18
		c.fillPolygon([]image.Point{{left, top}, {right, top}, {right - notch, top + h/2}, {right, bot}, {left, bot}, {left + notch, top + h/2}}, color.Black)
		c.bannerText(face, b.Title, left+notch, right-notch, top+6+shift, white)
		sub(top+6+titleH, "jost-regular", 21, white, left+notch, right-notch)
		return bot
	case "burst":
		// A black band with zigzag edges.
		face, _ := c.fitFace(b.titleFont("manrope"), b.Title, 46, c.contentW-40)
		m := face.Metrics()
		subH := 0
		if b.Subtitle != "" {
			subH = 24
		}
		zig := 9
		h := m.Ascent.Ceil() + m.Descent.Ceil() + 16 + subH
		top, bot := y+zig, y+zig+h
		pts := []image.Point{}
		for x := left; x <= right; x += zig * 2 {
			pts = append(pts, image.Point{x, top}, image.Point{min(x+zig, right), top - zig})
		}
		pts = append(pts, image.Point{right, top})
		for x := right; x >= left; x -= zig * 2 {
			pts = append(pts, image.Point{x, bot}, image.Point{max(x-zig, left), bot + zig})
		}
		pts = append(pts, image.Point{left, bot})
		c.fillPolygon(pts, color.Black)
		ty := c.bannerText(face, b.Title, left, right, top+6, white)
		sub(ty, "jost-regular", 21, white, left, right)
		return bot + zig
	case "stamp":
		// A thick rounded frame, heavy serif letters.
		face, _ := c.fitFace(b.titleFont("playfair"), b.Title, 44, c.contentW-60)
		top := y
		ty := c.bannerText(face, b.Title, left, right, top+11, black)
		ty = sub(ty, "jost", 20, black, left, right)
		bot := ty + 11
		for t := 0; t < 5; t++ {
			c.styledBoxAt(left+t, top+t, right-t, bot-t, "solid", 1, 16-t)
		}
		return bot
	case "frame":
		// A double-lined box, an elegant serif title.
		face, _ := c.fitFace(b.titleFont("cormorant"), b.Title, 54, c.contentW-60)
		top := y
		ty := c.bannerText(face, b.Title, left, right, top+10, black)
		if b.Subtitle != "" {
			c.fillRect(left+c.contentW/3, ty+2, right-c.contentW/3, ty+4, color.Black)
			ty = sub(ty+7, "lora", 20, black, left, right)
		}
		bot := ty + 11
		c.styledBoxAt(left, top, right, bot, "solid", 3, 0)
		c.styledBoxAt(left+7, top+7, right-7, bot-7, "solid", 1, 0)
		return bot
	case "ornament":
		// The title between rules, with diamonds.
		face, _ := c.fitFace(b.titleFont("playfair"), b.Title, 44, c.contentW-140)
		top := y
		m := face.Metrics()
		mid := top + (m.Ascent.Ceil()+m.Descent.Ceil())/2
		w := font.MeasureString(face, b.Title).Ceil()
		x0, x1 := left+(c.contentW-w)/2-16, left+(c.contentW+w)/2+16
		c.fillRect(left+14, mid-1, x0-12, mid+1, color.Black)
		c.fillRect(x1+12, mid-1, right-14, mid+1, color.Black)
		c.fillPolygon(diamond(left+7, mid, 7), color.Black)
		c.fillPolygon(diamond(right-7, mid, 7), color.Black)
		c.fillPolygon(diamond(x0-4, mid, 5), color.Black)
		c.fillPolygon(diamond(x1+4, mid, 5), color.Black)
		ty := c.bannerText(face, b.Title, left, right, top, black)
		return sub(ty+2, "lora", 20, black, left, right) + 4
	case "stars":
		// Rows of stars around a bold title.
		face, _ := c.fitFace(b.titleFont("manrope"), b.Title, 44, c.contentW-20)
		top := y
		row := func(cy int) {
			for i, x := 0, left+18; x < right; i, x = i+1, x+44 {
				r := 7
				if i%2 == 1 {
					r = 5
				}
				c.fillPolygon(star(x, cy, r), color.Black)
			}
		}
		row(top + 7)
		ty := c.bannerText(face, b.Title, left, right, top+17, black)
		ty = sub(ty, "jost", 20, black, left, right)
		row(ty + 10)
		return ty + 18
	}
	return y
}

// styledBoxAt is a box at the given corners, with corners rounded r (0 for
// square).
func (c *receiptCanvas) styledBoxAt(left, top, right, bottom int, style string, th, r int) {
	c.hLine(left+r, right-r, top, style, th)
	c.hLine(left+r, right-r, bottom-th, style, th)
	c.vLine(left, top+r, bottom-r, style, th)
	c.vLine(right-th, top+r, bottom-r, style, th)
	if r > 0 {
		c.arc(left+r, top+r, r, th, math.Pi, 1.5*math.Pi)
		c.arc(right-r, top+r, r, th, 1.5*math.Pi, 2*math.Pi)
		c.arc(left+r, bottom-r, r, th, 0.5*math.Pi, math.Pi)
		c.arc(right-r, bottom-r, r, th, 0, 0.5*math.Pi)
	}
}

// renderBannerPreview draws one banner alone, as it prints, for the
// designer's gallery.
func renderBannerPreview(b receiptBanner) ([]byte, error) {
	c := &receiptCanvas{
		img: image.NewRGBA(image.Rect(0, 0, raster80mmWidth, 400)), width: raster80mmWidth,
		margin: 24, contentW: raster80mmWidth - 48, faces: map[string]font.Face{},
	}
	for i := range c.img.Pix {
		c.img.Pix[i] = 0xff
	}
	bottom := c.drawBanner(16, b) + 16
	if bottom > 400 {
		bottom = 400
	}
	return encodeBilevelPNGLevel(c.img.SubImage(image.Rect(0, 0, raster80mmWidth, bottom)).(*image.RGBA), png.BestSpeed)
}
