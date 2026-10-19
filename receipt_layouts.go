package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// Item layouts. Each takes the canvas and the y to start at, prints its own
// column header and every item, and returns the new y plus the undiscounted
// subtotal and the total discount (the totals block prints both).

// itemAmounts returns the line's gross, its discount, and the net total.
func itemAmounts(item invoiceItem) (gross, discount, net float64) {
	gross = item.Qty * item.Price
	discount = gross * item.Discount / 100
	return gross, discount, gross - discount
}

func dollars(v float64) string { return fmt.Sprintf("$%.2f", v) }

// plainAmount is dollars() without the symbol, for tape-style columns that
// say what the currency is once, in the totals.
func plainAmount(v float64) string { return fmt.Sprintf("%.2f", v) }

func percent(v float64) string { return fmt.Sprintf("%.0f%%", v) }

func qtyText(q float64) string { return fmt.Sprintf("%.0f", q) }

// variantText joins an item's colour and size ("Navy · M"), or "".
func variantText(item invoiceItem) string {
	var parts []string
	for _, part := range []string{item.Color, item.Size} {
		part = strings.TrimSpace(part)
		if part != "" && part != "-" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " · ")
}

// wrapText wraps name to the width at the given face; the first line may be
// narrower than the rest to leave room for an amount beside it.
func (c *receiptCanvas) wrapText(name string, size float64, role fontRole, firstW, fullW int) []string {
	face, err := c.faceRole(size, role)
	if err != nil {
		return []string{name}
	}
	return wrapRasterItemName(face, name, firstW, fullW)
}

// --- column grid (classic A/B, modern table) -------------------------------

type gridConfig struct {
	midHeader string
	midSample string
	mid       func(item invoiceItem, discount float64) string
	// subline is printed under an item (classic B's discount line); "" skips.
	subline  func(item invoiceItem, discount float64) string
	nameRole fontRole
	numRole  fontRole
	sumRole  fontRole
	size     float64
	// headerBand draws the column header white-on-black; rowRule separates
	// the items with a thin line.
	headerBand bool
	rowRule    bool
}

func (c *receiptCanvas) drawGrid(y int, cfg gridConfig) (int, float64, float64) {
	right := c.margin + c.contentW
	hdrSize := c.data.BodySize
	pad := 10
	qtyHdr, _ := c.measureRole("QTY", hdrSize, roleBodyBold)
	midHdr, _ := c.measureRole(cfg.midHeader, hdrSize, roleBodyBold)
	midSmp, _ := c.measureRole(cfg.midSample, cfg.size, cfg.numRole)
	sumSmp, _ := c.measureRole("$000.00", cfg.size, cfg.sumRole)
	totalW := maxInt(int(0.21*float64(c.contentW)), maxInt(sumSmp, qtyHdr)+pad)
	midW := maxInt(int(0.16*float64(c.contentW)), maxInt(midHdr, midSmp)+pad)
	qtyW := maxInt(int(0.10*float64(c.contentW)), qtyHdr+pad)
	totalR := right
	midR := right - totalW
	qtyR := midR - midW
	nameW := qtyR - qtyW - c.margin
	// On narrow paper the middle column would squeeze names to a few
	// characters, so it folds into a second line under the item instead.
	narrow := nameW < c.contentW*38/100
	if narrow {
		midW = 0
		qtyR = midR
		nameW = qtyR - qtyW - c.margin
	}

	// header
	if cfg.headerBand {
		h := c.lineHeight(hdrSize, roleBodyBold)
		c.fillRect(c.margin, y, right, y+h+10, color.Black)
		c.textRole(c.margin+6, y+5, "ITEM", hdrSize, roleBodyBold, image.White)
		c.rightRole(qtyR, y+5, "QTY", hdrSize, roleBodyBold, image.White)
		if !narrow {
			c.rightRole(midR, y+5, cfg.midHeader, hdrSize, roleBodyBold, image.White)
		}
		c.rightRole(totalR-6, y+5, "TOTAL", hdrSize, roleBodyBold, image.White)
		y += h + 10 + 10
	} else {
		c.textRole(c.margin, y, "ITEM", hdrSize, roleBodyBold, image.Black)
		c.rightRole(qtyR, y, "QTY", hdrSize, roleBodyBold, image.Black)
		if !narrow {
			c.rightRole(midR, y, cfg.midHeader, hdrSize, roleBodyBold, image.Black)
		}
		hb, _ := c.rightRole(totalR, y, "TOTAL", hdrSize, roleBodyBold, image.Black)
		y = c.ruleLine(hb+6) + 10
	}

	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := c.wrapText(item.Name, cfg.size, cfg.nameRole, nameW, nameW)
		bottom := y
		for i, line := range lines {
			bottom, _ = c.textRole(c.margin, y, line, cfg.size, cfg.nameRole, image.Black)
			if i == 0 {
				c.rightRole(qtyR, y, qtyText(item.Qty), cfg.size, cfg.numRole, image.Black)
				if !narrow {
					c.rightRole(midR, y, cfg.mid(item, discount), cfg.size, cfg.numRole, image.Black)
				}
				c.rightRole(totalR, y, dollars(net), cfg.size, cfg.sumRole, image.Black)
			}
			if i < len(lines)-1 {
				y = bottom + 2
			}
		}
		y = bottom + 4
		subline := cfg.subline
		if narrow {
			// what the hidden middle column carried
			subline = func(item invoiceItem, discount float64) string {
				text := ""
				if cfg.midHeader == "PRICE" {
					text = "@ " + dollars(item.Price)
				}
				if item.Discount > 0 {
					text += fmt.Sprintf(" -%.0f%% (-$%.2f)", item.Discount, discount)
				}
				return strings.TrimSpace(text)
			}
		}
		if subline != nil {
			if text := subline(item, discount); text != "" {
				y, _ = c.textRole(c.margin+12, y, text, 10, roleBody, image.Black)
				y += 4
			}
		}
		if cfg.rowRule {
			c.fillRect(c.margin, y+2, right, y+3, color.Gray{Y: 150})
			y += 8
		} else {
			y += 5
		}
	}
	return y, subtotal, discountTotal
}

func discountOrDash(item invoiceItem, _ float64) string {
	if item.Discount > 0 {
		return percent(item.Discount)
	}
	return "-"
}

func drawClassicColumns(c *receiptCanvas, y int) (int, float64, float64) {
	return c.drawGrid(y, gridConfig{
		midHeader: "DISC", midSample: "100%", mid: discountOrDash,
		nameRole: roleName, numRole: roleBody, sumRole: roleBodyBold, size: 10.5,
	})
}

func drawClassicPriceColumns(c *receiptCanvas, y int) (int, float64, float64) {
	return c.drawGrid(y, gridConfig{
		midHeader: "PRICE", midSample: "$000.00",
		mid: func(item invoiceItem, _ float64) string { return dollars(item.Price) },
		subline: func(item invoiceItem, discount float64) string {
			if item.Discount > 0 {
				return fmt.Sprintf("-%.0f%% (-$%.2f)", item.Discount, discount)
			}
			return ""
		},
		nameRole: roleName, numRole: roleBody, sumRole: roleBodyBold, size: 10.5,
	})
}

// drawClassicDetailed is the original layout: name and total, the code, and
// a quantity line.
func drawClassicDetailed(c *receiptCanvas, y int) (int, float64, float64) {
	y, _ = c.twoCol(y, "ITEM", "TOTAL", c.data.BodySize, true)
	y = c.ruleLine(y+6) + 10
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		y, _ = c.itemRow(y, item.Name, dollars(net), 11)
		y += 5
		if item.Code != "" {
			y, _ = c.indented(y, "Code: "+item.Code, 10)
			y += 3
		}
		qty := fmt.Sprintf("%.0f x $%.2f", item.Qty, item.Price)
		if item.Discount > 0 {
			qty += fmt.Sprintf("  (-%.0f%% = -$%.2f)", item.Discount, discount)
		}
		y, _ = c.indented(y, qty, 10)
		y += 7
	}
	return y, subtotal, discountTotal
}

// --- monospaced tapes (supermarket, typewriter) ----------------------------

// tape lays text on a fixed character grid so columns line up the way a
// register tape's do, whatever the paper width.
type tape struct {
	c     *receiptCanvas
	size  float64
	charW float64
	cols  int
}

// minTapeCols is the narrowest a tape may get; on narrow paper the type
// shrinks to keep this many characters per line rather than leave names a
// few letters wide.
const minTapeCols = 30

func (c *receiptCanvas) newTape(size float64) tape {
	for {
		w, _ := c.measureRole("MMMMMMMMMM", size, roleBody)
		charW := float64(w) / 10
		if charW < 1 {
			charW = 1
		}
		cols := int(float64(c.contentW) / charW)
		if cols >= minTapeCols || size <= 6 {
			return tape{c: c, size: size, charW: charW, cols: cols}
		}
		size -= 0.5
	}
}

func (t tape) x(col int) int { return t.c.margin + int(float64(col)*t.charW+0.5) }

// put draws text starting at col; putRight ends it at endCol (exclusive).
func (t tape) put(y, col int, text string, role fontRole) int {
	bottom, _ := t.c.textRole(t.x(col), y, text, t.size, role, image.Black)
	return bottom
}

func (t tape) putRight(y, endCol int, text string, role fontRole) int {
	return t.put(y, endCol-len([]rune(text)), text, role)
}

// wrap breaks text into lines of at most width characters.
func wrapColumns(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	line := ""
	flush := func() {
		lines = append(lines, line)
		line = ""
	}
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		for len(runes) > width {
			if line != "" {
				flush()
			}
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		word = string(runes)
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len(runes) <= width:
			line += " " + word
		default:
			flush()
			line = word
		}
	}
	if line != "" || len(lines) == 0 {
		flush()
	}
	return lines
}

func (t tape) rule(y int) int { return t.c.ruleLine(y) + 6 }

// step is the vertical advance after one tape line.
func (t tape) step(bottom int) int { return bottom + 3 }

func drawSupermarketRegister(c *receiptCanvas, y int) (int, float64, float64) {
	t := c.newTape(11)
	amtW := 8
	t.put(y, 0, "ITEM", roleBodyBold)
	y = t.step(t.putRight(y, t.cols, "AMOUNT", roleBodyBold))
	y = t.rule(y + 2)
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := wrapColumns(strings.ToUpper(item.Name), t.cols-amtW-1)
		for i, line := range lines {
			bottom := t.put(y, 0, line, roleBody)
			if i == len(lines)-1 {
				t.putRight(y, t.cols, plainAmount(net), roleBodyBold)
			}
			y = t.step(bottom)
		}
		if item.Qty != 1 || item.Discount > 0 {
			detail := fmt.Sprintf("  %s @ %s", qtyText(item.Qty), plainAmount(item.Price))
			bottom := t.put(y, 0, detail, roleBody)
			if item.Discount > 0 {
				t.putRight(y, t.cols, fmt.Sprintf("DISC %s -%s", percent(item.Discount), plainAmount(discount)), roleBody)
			}
			y = t.step(bottom)
		}
		y += 3
	}
	return y, subtotal, discountTotal
}

func drawSupermarketQuantity(c *receiptCanvas, y int) (int, float64, float64) {
	t := c.newTape(11)
	amtW, qtyW := 8, 3
	t.put(y, 0, "QTY", roleBodyBold)
	t.put(y, qtyW+1, "ITEM", roleBodyBold)
	y = t.step(t.putRight(y, t.cols, "TOTAL", roleBodyBold))
	y = t.rule(y + 2)
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := wrapColumns(strings.ToUpper(item.Name), t.cols-qtyW-1-amtW-1)
		for i, line := range lines {
			if i == 0 {
				t.putRight(y, qtyW, qtyText(item.Qty), roleBody)
			}
			bottom := t.put(y, qtyW+1, line, roleBody)
			if i == len(lines)-1 {
				t.putRight(y, t.cols, plainAmount(net), roleBodyBold)
			}
			y = t.step(bottom)
		}
		if item.Discount > 0 {
			bottom := t.put(y, qtyW+1, fmt.Sprintf("DISC %s", percent(item.Discount)), roleBody)
			t.putRight(y, t.cols, "-"+plainAmount(discount), roleBody)
			y = t.step(bottom)
		}
		y += 3
	}
	return y, subtotal, discountTotal
}

func drawSupermarketCodes(c *receiptCanvas, y int) (int, float64, float64) {
	t := c.newTape(11)
	amtW := 8
	t.put(y, 0, "ITEM / CODE", roleBodyBold)
	y = t.step(t.putRight(y, t.cols, "AMOUNT", roleBodyBold))
	y = t.rule(y + 2)
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := wrapColumns(strings.ToUpper(item.Name), t.cols-amtW-1)
		for i, line := range lines {
			bottom := t.put(y, 0, line, roleBody)
			if i == len(lines)-1 {
				t.putRight(y, t.cols, plainAmount(net), roleBodyBold)
			}
			y = t.step(bottom)
		}
		code := item.Code
		if code == "" {
			code = item.Barcode
		}
		detail := fmt.Sprintf("  %s", code)
		bottom := t.put(y, 0, detail, roleBody)
		right := fmt.Sprintf("%s x %s", qtyText(item.Qty), plainAmount(item.Price))
		if item.Discount > 0 {
			right += fmt.Sprintf("  -%s", percent(item.Discount))
		}
		t.putRight(y, t.cols, right, roleBody)
		y = t.step(bottom) + 3
	}
	return y, subtotal, discountTotal
}

func drawTypewriterTape(c *receiptCanvas, y int) (int, float64, float64) {
	t := c.newTape(9.5)
	qtyW, priceW, totalW := 3, 8, 8
	nameStart := qtyW + 1
	priceEnd := t.cols - totalW - 1
	t.putRight(y, qtyW, "QTY", roleBodyBold)
	t.put(y, nameStart, "ITEM", roleBodyBold)
	t.putRight(y, priceEnd, "PRICE", roleBodyBold)
	y = t.step(t.putRight(y, t.cols, "TOTAL", roleBodyBold))
	y = t.rule(y + 2)
	nameW := priceEnd - priceW - nameStart
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := wrapColumns(strings.ToUpper(item.Name), nameW)
		for i, line := range lines {
			if i == 0 {
				t.putRight(y, qtyW, qtyText(item.Qty), roleBody)
				t.putRight(y, priceEnd, plainAmount(item.Price), roleBody)
				t.putRight(y, t.cols, plainAmount(net), roleBodyBold)
			}
			y = t.step(t.put(y, nameStart, line, roleBody))
		}
		if item.Discount > 0 {
			bottom := t.put(y, nameStart, fmt.Sprintf("LESS %s", percent(item.Discount)), roleBody)
			t.putRight(y, t.cols, "-"+plainAmount(discount), roleBody)
			y = t.step(bottom)
		}
		y += 3
	}
	return y, subtotal, discountTotal
}

func drawTypewriterItemized(c *receiptCanvas, y int) (int, float64, float64) {
	t := c.newTape(9.5)
	amtW := 8
	t.put(y, 0, "ITEM", roleBodyBold)
	y = t.step(t.putRight(y, t.cols, "AMOUNT", roleBodyBold))
	y = t.rule(y + 2)
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		lines := wrapColumns(strings.ToUpper(item.Name), t.cols-amtW-1)
		for i, line := range lines {
			bottom := t.put(y, 0, line, roleBodyBold)
			if i == len(lines)-1 {
				t.putRight(y, t.cols, plainAmount(net), roleBodyBold)
			}
			y = t.step(bottom)
		}
		bottom := t.put(y, 2, fmt.Sprintf("%s @ %s", qtyText(item.Qty), plainAmount(item.Price)), roleBody)
		if item.Discount > 0 {
			t.putRight(y, t.cols, plainAmount(gross), roleBody)
			y = t.step(bottom)
			bottom = t.put(y, 2, fmt.Sprintf("LESS %s", percent(item.Discount)), roleBody)
			t.putRight(y, t.cols, "-"+plainAmount(discount), roleBody)
		}
		y = t.step(bottom) + 4
	}
	return y, subtotal, discountTotal
}

// --- boutique --------------------------------------------------------------

func drawBoutiqueCentered(c *receiptCanvas, y int) (int, float64, float64) {
	var subtotal, discountTotal float64
	for i, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		for _, line := range c.wrapText(item.Name, 14, roleName, c.contentW, c.contentW) {
			y, _ = c.centerRole(y, line, 14, roleName)
		}
		y += 1
		if variant := variantText(item); variant != "" {
			y, _ = c.centerRole(y, variant, 10, roleBody)
			y += 2
		}
		calc := fmt.Sprintf("%s × %s", qtyText(item.Qty), dollars(item.Price))
		if item.Discount > 0 {
			calc += fmt.Sprintf("  ·  -%s", percent(item.Discount))
		}
		calc += "  ·  " + dollars(net)
		y, _ = c.centerRole(y, calc, 10.5, roleBodyBold)
		y += 10
		if i < len(c.data.Items)-1 {
			mid := c.margin + c.contentW/2
			c.diamond(mid, y, 2)
			y += 12
		}
	}
	return y, subtotal, discountTotal
}

func drawBoutiqueLedger(c *receiptCanvas, y int) (int, float64, float64) {
	right := c.margin + c.contentW
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		amount := dollars(net)
		amtW, _ := c.measureRole(amount, 11.5, roleBodyBold)
		lines := c.wrapText(item.Name, 14, roleName, c.contentW-amtW-16, c.contentW)
		bottom := y
		for i, line := range lines {
			bottom, _ = c.textRole(c.margin, y, line, 14, roleName, image.Black)
			if i == 0 {
				c.rightRole(right, y+2, amount, 11.5, roleBodyBold, image.Black)
			}
			if i < len(lines)-1 {
				y = bottom
			}
		}
		y = bottom + 1
		detail := fmt.Sprintf("%s × %s", qtyText(item.Qty), dollars(item.Price))
		if item.Discount > 0 {
			detail += fmt.Sprintf(" · -%s", percent(item.Discount))
		}
		if variant := variantText(item); variant != "" {
			detail = variant + " · " + detail
		}
		for _, line := range c.wrapText(detail, 10, roleBody, c.contentW, c.contentW) {
			y, _ = c.textRole(c.margin, y, line, 10, roleBody, image.Black)
		}
		y += 12
	}
	return y, subtotal, discountTotal
}

// --- modern ----------------------------------------------------------------

func drawModernClean(c *receiptCanvas, y int) (int, float64, float64) {
	right := c.margin + c.contentW
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		amount := dollars(net)
		amtW, _ := c.measureRole(amount, 11, roleName)
		lines := c.wrapText(item.Name, 11, roleName, c.contentW-amtW-16, c.contentW)
		bottom := y
		for i, line := range lines {
			bottom, _ = c.textRole(c.margin, y, line, 11, roleName, image.Black)
			if i == 0 {
				c.rightRole(right, y, amount, 11, roleName, image.Black)
			}
			if i < len(lines)-1 {
				y = bottom + 2
			}
		}
		y = bottom + 3
		detail := fmt.Sprintf("%s × %s", qtyText(item.Qty), dollars(item.Price))
		if variant := variantText(item); variant != "" {
			detail = strings.ReplaceAll(variant, " · ", " / ") + "  ·  " + detail
		}
		bottom, _ = c.textRole(c.margin, y, detail, 9.5, roleBody, image.Black)
		if item.Discount > 0 {
			c.rightRole(right, y, fmt.Sprintf("-%s  (-%s)", percent(item.Discount), dollars(discount)), 9.5, roleBody, image.Black)
		}
		y = bottom + 12
	}
	return y, subtotal, discountTotal
}

func drawModernTable(c *receiptCanvas, y int) (int, float64, float64) {
	return c.drawGrid(y, gridConfig{
		midHeader: "DISC", midSample: "100%", mid: discountOrDash,
		nameRole: roleName, numRole: roleBody, sumRole: roleBodyBold, size: 10.5,
		headerBand: true, rowRule: true,
	})
}

// --- bistro ----------------------------------------------------------------

// leaders draws a row of dots from x0 to x1 at the text's baseline area.
func (c *receiptCanvas) leaders(x0, x1, y, lineH int) {
	if x1-x0 < 12 {
		return
	}
	dotY := y + lineH*3/4
	for x := x0 + 4; x < x1-4; x += 8 {
		c.fillRect(x, dotY, x+2, dotY+2, color.Black)
	}
}

func drawBistroMenu(c *receiptCanvas, y int) (int, float64, float64) {
	right := c.margin + c.contentW
	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		amount := dollars(net)
		amtW, _ := c.measureRole(amount, 11, roleBodyBold)
		lines := c.wrapText(item.Name, 11.5, roleName, c.contentW-amtW-24, c.contentW-amtW-24)
		lineH := c.lineHeight(11.5, roleName)
		bottom := y
		for i, line := range lines {
			bottom, _ = c.textRole(c.margin, y, line, 11.5, roleName, image.Black)
			if i == len(lines)-1 {
				nameW, _ := c.measureRole(line, 11.5, roleName)
				c.leaders(c.margin+nameW+6, right-amtW-6, y, lineH)
				c.rightRole(right, y, amount, 11, roleBodyBold, image.Black)
			}
			if i < len(lines)-1 {
				y = bottom + 1
			}
		}
		y = bottom + 3
		if item.Qty != 1 || item.Discount > 0 {
			detail := fmt.Sprintf("%s × %s", qtyText(item.Qty), dollars(item.Price))
			if item.Discount > 0 {
				detail += fmt.Sprintf("  ·  %s off", percent(item.Discount))
			}
			y, _ = c.textRole(c.margin+14, y, detail, 9.5, roleBody, image.Black)
			y += 2
		}
		y += 9
	}
	return y, subtotal, discountTotal
}

func drawBistroQuantity(c *receiptCanvas, y int) (int, float64, float64) {
	right := c.margin + c.contentW
	var subtotal, discountTotal float64
	qtyColW, _ := c.measureRole("00 ×", 11, roleBodyBold)
	nameX := c.margin + qtyColW + 10
	for _, item := range c.data.Items {
		gross, discount, net := itemAmounts(item)
		subtotal += gross
		discountTotal += discount
		amount := dollars(net)
		amtW, _ := c.measureRole(amount, 11, roleBodyBold)
		nameW := right - amtW - 24 - nameX
		lines := c.wrapText(item.Name, 11.5, roleName, nameW, nameW)
		lineH := c.lineHeight(11.5, roleName)
		bottom := y
		c.textRole(c.margin, y, qtyText(item.Qty)+" ×", 11, roleBodyBold, image.Black)
		for i, line := range lines {
			bottom, _ = c.textRole(nameX, y, line, 11.5, roleName, image.Black)
			if i == len(lines)-1 {
				nameW, _ := c.measureRole(line, 11.5, roleName)
				c.leaders(nameX+nameW+6, right-amtW-6, y, lineH)
				c.rightRole(right, y, amount, 11, roleBodyBold, image.Black)
			}
			if i < len(lines)-1 {
				y = bottom + 1
			}
		}
		y = bottom + 3
		if item.Discount > 0 {
			y, _ = c.textRole(nameX, y, fmt.Sprintf("%s off", percent(item.Discount)), 9.5, roleBody, image.Black)
			y += 2
		}
		y += 9
	}
	return y, subtotal, discountTotal
}
