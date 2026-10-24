package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
)

// ruleLine draws a horizontal divider in the theme's style and returns the y
// below it. The classic dotted rule is the original c.rule.
func (c *receiptCanvas) ruleLine(y int) int {
	left, right := c.margin, c.margin+c.contentW
	switch c.style.Rule {
	case ruleDashed:
		for x := left; x < right; x += 22 {
			c.fillRect(x, y, minInt(x+14, right), y+2, color.Black)
		}
		return y + 2
	case ruleSolid:
		c.fillRect(left, y, right, y+2, color.Black)
		return y + 2
	case ruleDouble:
		c.fillRect(left, y, right, y+1, color.Black)
		c.fillRect(left, y+4, right, y+5, color.Black)
		return y + 5
	case ruleOrnament:
		mid := (left + right) / 2
		c.fillRect(left, y+4, mid-16, y+5, color.Black)
		c.fillRect(mid+16, y+4, right, y+5, color.Black)
		c.diamond(mid, y+4, 5)
		return y + 9
	}
	return c.rule(y)
}

func (c *receiptCanvas) fillRect(x0, y0, x1, y1 int, fill color.Color) {
	rect := image.Rect(x0, y0, x1, y1).Intersect(c.img.Bounds())
	draw.Draw(c.img, rect, image.NewUniform(fill), image.Point{}, draw.Src)
}

// diamond fills a small diamond centred on (cx, cy).
func (c *receiptCanvas) diamond(cx, cy, radius int) {
	for dy := -radius; dy <= radius; dy++ {
		span := radius - absInt(dy)
		c.fillRect(cx-span, cy+dy, cx+span+1, cy+dy+1, color.Black)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// lineHeight is the height c.textRole advances for a face, so callers can
// size a bar around a line of text.
func (c *receiptCanvas) lineHeight(size float64, role fontRole) int {
	face, err := c.faceRole(size, role)
	if err != nil {
		return int(size * 3)
	}
	return face.Metrics().Height.Ceil()
}

// rightRole draws value so it ends at xRight.
func (c *receiptCanvas) rightRole(xRight, y int, value string, size float64, role fontRole, ink image.Image) (int, error) {
	width, err := c.measureRole(value, size, role)
	if err != nil {
		return y, err
	}
	return c.textRole(xRight-width, y, value, size, role, ink)
}

// centerRole centres value across the content width.
func (c *receiptCanvas) centerRole(y int, value string, size float64, role fontRole) (int, error) {
	width, err := c.measureRole(value, size, role)
	if err != nil {
		return y, err
	}
	return c.textRole(c.margin+(c.contentW-width)/2, y, value, size, role, image.Black)
}

// drawMeta prints customer, salesperson and date. The classic look stacks
// "Label: value" lines; MetaRows puts the label left and the value right;
// MetaCentered centres the stacked lines.
func (c *receiptCanvas) drawMeta(y int) int {
	pairs := [][2]string{
		{"Customer", fallbackReceiptValue(c.data.Customer, "Walk-in")},
		{"Salesperson", formatReceiptSalesperson(c.data.Salesperson)},
		{"Date", currentReceiptDate()},
	}
	size := c.data.BodySize - 1
	for _, pair := range pairs {
		switch {
		case c.style.MetaRows:
			c.textRole(c.margin, y, pair[0], size, roleBody, image.Black)
			y, _ = c.rightRole(c.margin+c.contentW, y, pair[1], size, roleBodyBold, image.Black)
		case c.style.MetaCentered:
			y, _ = c.centerRole(y, pair[0]+": "+pair[1], size, roleBody)
		default:
			y, _ = c.left(y, pair[0]+": "+pair[1], size, false)
		}
		y += c.gap(8)
	}
	// Room before the rule under the block, as there is above the first line.
	return y + c.gap(8)
}

// drawTotal prints the TOTAL line (and the rounding-currency equivalent) in
// the theme's style, returning the y below it.
func (c *receiptCanvas) drawTotal(y int, grandTotal float64) int {
	amount := fmt.Sprintf("$%.2f", grandTotal)
	var second [2]string
	if currency := c.data.RoundingCurrency; currency.UnitsPerUsdMicros > 0 {
		if rounded, err := roundCurrencyUnitsFromCents(c.data.GrandTotalCents, currency.UnitsPerUsdMicros, currency.CashRoundingUnit, c.data.RoundingMode); err == nil {
			second = [2]string{currency.Code + " TOTAL", formatInt(rounded) + " " + currency.Symbol}
		}
	}
	right := c.margin + c.contentW

	switch c.style.Total {
	case totalBand:
		const pad = 8
		size := 14.0
		h := c.lineHeight(size, roleBodyBold)
		top := y + 4
		c.fillRect(c.margin, top, right, top+h+2*pad, color.Black)
		c.textRole(c.margin+pad, top+pad, "TOTAL", size, roleBodyBold, image.White)
		c.rightRole(right-pad, top+pad, amount, size, roleBodyBold, image.White)
		y = top + h + 2*pad + 8
		if second[0] != "" {
			y, _ = c.twoCol(y, second[0], second[1], 11, true)
			y += 8
		}
		return y
	case totalRuled:
		y = c.ruleLine(y) + 10
		size := 15.0
		// Display serifs set figures old-style (descending digits); the
		// amount is read at a glance, so it stays in the body face.
		labelRole := roleBodyBold
		if c.style.Fonts != nil {
			labelRole = roleTitleBold
		}
		c.textRole(c.margin, y, "TOTAL", size, labelRole, image.Black)
		y, _ = c.rightRole(right, y, amount, size, roleBodyBold, image.Black)
		y += c.gap(8)
		if second[0] != "" {
			y, _ = c.twoCol(y, second[0], second[1], 11, true)
			y += 8
		}
		return c.ruleLine(y) + 4
	}

	// Room inside the box above and below the figures, and a little outside
	// it before the next section starts.
	boxTop := c.ruleLine(y) + 16
	y = boxTop + 16
	y, _ = c.twoCol(y, "TOTAL", amount, 13, true)
	y += 8
	if second[0] != "" {
		y, _ = c.twoCol(y, second[0], second[1], 11, true)
		y += 8
	}
	y += 14
	c.dottedBox(boxTop, y-boxTop)
	return y + 8
}

func upperIf(on bool, value string) string {
	if on {
		return strings.ToUpper(value)
	}
	return value
}
