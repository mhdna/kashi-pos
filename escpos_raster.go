package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	raster80mmWidth  = 576
	raster58mmWidth  = 384
	rasterBandHeight = 256
	rasterMaxHeight  = 10000
)

type receiptCanvas struct {
	data     printReceiptData
	img      *image.RGBA
	width    int
	margin   int
	contentW int
	faces    map[string]font.Face
}

func buildESCPOSRasterReceipt(data printReceiptData) ([]byte, error) {
	if data.TitleSize <= 0 {
		data.TitleSize = 21
	}
	if data.BodySize <= 0 {
		data.BodySize = 11
	}
	width := raster80mmWidth
	if data.ReceiptWidth > 0 && data.ReceiptWidth <= 58 {
		width = raster58mmWidth
	}
	estimatedHeight := 1900 + len(data.Items)*110 + len(data.SocialPlatforms)*60 + len(data.ReturnPolicy)*2
	if estimatedHeight > rasterMaxHeight {
		return nil, fmt.Errorf("receipt is too long to print safely")
	}

	c := &receiptCanvas{
		data:     data,
		img:      image.NewRGBA(image.Rect(0, 0, width, estimatedHeight)),
		width:    width,
		margin:   24,
		contentW: width - 48,
		faces:    make(map[string]font.Face),
	}
	draw.Draw(c.img, c.img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	y, err := c.drawReceipt()
	if err != nil {
		return nil, err
	}
	if y <= 0 || y > c.img.Bounds().Dy() {
		return nil, fmt.Errorf("receipt raster height is invalid: %d", y)
	}

	cropped := image.NewRGBA(image.Rect(0, 0, width, y))
	draw.Draw(cropped, cropped.Bounds(), c.img, image.Point{}, draw.Src)
	payload := encodeRasterBands(cropped, data.Cutoff)
	if err := validateRasterPayload(payload, width); err != nil {
		return nil, fmt.Errorf("unsafe printer payload rejected: %w", err)
	}
	return payload, nil
}

func (c *receiptCanvas) drawReceipt() (int, error) {
	y := 0
	var err error
	if y, err = c.center(y, c.data.StoreName, c.data.TitleSize, true, true); err != nil {
		return 0, err
	}
	y += 22
	y, _ = c.center(y, c.data.Branch, c.data.BodySize-1, true, false)
	y += 10
	if c.data.Description != "" {
		y, _ = c.centerWrapped(y, c.data.Description, c.data.BodySize-1, 8)
	}
	y = c.barcode(y+6, c.data.ReceiptNo) + 6
	y, _ = c.twoCol(y, "Inv# "+c.data.ReceiptNo, currentReceiptDate(), c.data.BodySize-1, true)
	y += 8
	phone := c.data.BranchPhone
	if phone == "" {
		phone = c.data.Phone
	}
	contact := strings.TrimSpace(strings.Join(filterEmpty(phone, c.data.Website), "  •  "))
	if contact != "" {
		y, _ = c.center(y, contact, c.data.BodySize-1, true, false)
		y += 8
	}
	y = c.rule(y) + 10

	metadata := []string{
		"Sold by: " + fallbackReceiptValue(c.data.Salesperson, "Not specified"),
		"Customer: " + fallbackReceiptValue(c.data.Customer, "Walk-in"),
	}
	for _, line := range metadata {
		y, _ = c.left(y, line, c.data.BodySize-1, false)
		y += 8
	}
	if strings.TrimSpace(c.data.Notes) != "" {
		y += 6
		y, _ = c.left(y, "Note: "+strings.TrimSpace(c.data.Notes), c.data.BodySize-1, false)
		y += 8
	}
	y = c.rule(y) + 10
	y, _ = c.twoCol(y, "ITEM", "TOTAL", c.data.BodySize, true)
	y = c.rule(y+6) + 10

	var subtotal, discountTotal float64
	for _, item := range c.data.Items {
		gross := item.Qty * item.Price
		discount := gross * item.Discount / 100
		lineTotal := gross - discount
		subtotal += gross
		discountTotal += discount
		y, _ = c.itemRow(y, item.Name, fmt.Sprintf("$%.2f", lineTotal), 11)
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

	y = c.rule(y) + 10
	grandTotal := float64(c.data.GrandTotalCents) / 100
	tax := grandTotal - (subtotal - discountTotal)
	rows := [][2]string{{"Subtotal", fmt.Sprintf("$%.2f", subtotal)}}
	if discountTotal > 0 {
		rows = append(rows, [2]string{"Discount", fmt.Sprintf("-$%.2f", discountTotal)})
	}
	if math.Abs(tax) > 0.005 {
		rows = append(rows, [2]string{"Tax", fmt.Sprintf("$%.2f", tax)})
	}
	for _, row := range rows {
		y, _ = c.twoCol(y, row[0], row[1], 11, true)
		y += 7
	}
	y = c.rule(y) + 12
	y = c.dottedBox(y, 27)
	y += 6
	y, _ = c.twoCol(y, "TOTAL", fmt.Sprintf("$%.2f", grandTotal), 13, true)
	y += 8
	rate := c.data.ExchangeRate
	if rate <= 0 {
		rate = 90000
	}
	lbp, roundErr := roundLBPFromCents(c.data.GrandTotalCents, rate, c.data.RoundingMode)
	if roundErr != nil {
		lbp = int64(grandTotal * float64(rate))
	}
	y, _ = c.twoCol(y, "LBP TOTAL", formatInt(lbp)+" L.L.", 11, true)
	y += 12

	if len(c.data.Payments) > 0 {
		y = c.rule(y) + 10
		y, _ = c.center(y, "PAYMENT DETAILS", c.data.BodySize-1, true, false)
		y += 8
		for _, payment := range c.data.Payments {
			label := payment.AccountName
			if label == "" {
				label = "Cash"
			}
			if payment.EntryType == "change" {
				label = "Change returned - " + label
			} else {
				label = "Cash received - " + label
			}
			y, _ = c.twoCol(y, label, formatReceiptPayment(payment, rate, c.data.RoundingMode), 10, true)
			y += 7
		}
	}

	y = c.rule(y) + 10
	y, _ = c.center(y, "RETURN & EXCHANGE POLICY", c.data.BodySize-1.5, true, false)
	y += 8
	policy := c.data.ReturnPolicy
	if policy == "" {
		policy = "Exchanges accepted within 3 days."
	}
	y, _ = c.centerWrapped(y, policy, c.data.BodySize-2.5, 8)
	y = c.rule(y+6) + 10

	socials := socialFooterItems(c.data.SocialPlatforms, c.data.SocialHandles)
	if len(socials) > 0 {
		var rowBottom int
		if rowBottom, err = c.socialFooterRow(y, socials); err != nil {
			return 0, err
		}
		y = rowBottom + 10
		// Match the PDF receipt: show one centered handle beneath the icon row.
		if y, err = c.center(y, socials[0].handle, c.data.BodySize-1.5, false, false); err != nil {
			return 0, err
		}
		y += 8
	}
	y += 10
	y, _ = c.center(y, "Thank you for shopping with us!", c.data.BodySize-1, true, false)
	return y + 24, nil
}

func (c *receiptCanvas) face(size float64, bold, title bool) (font.Face, error) {
	key := fmt.Sprintf("%.2f:%t:%t", size, bold, title)
	if face := c.faces[key]; face != nil {
		return face, nil
	}
	fontData := gomono.TTF
	if bold {
		fontData = gobold.TTF
	}
	if title {
		fontData = goregular.TTF
	}
	if (title && c.data.Font == "Belleza") || (!title && c.data.BodyFont == "Belleza") {
		fontData = bellezaFont
	} else if !title && (c.data.BodyFont == "CascadiaMono" || c.data.BodyFont == "Helvetica" || c.data.BodyFont == "Courier" || c.data.BodyFont == "Printer Font A") {
		fontData = cascadiaMonoRegular
		if bold {
			fontData = cascadiaMonoBold
		}
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse receipt font: %w", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size * 2.6, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("create receipt font: %w", err)
	}
	c.faces[key] = face
	return face, nil
}

func (c *receiptCanvas) text(x, y int, value string, size float64, bold, title bool) (int, error) {
	face, err := c.face(size, bold, title)
	if err != nil {
		return y, err
	}
	metrics := face.Metrics()
	drawer := font.Drawer{Dst: c.img, Src: image.Black, Face: face, Dot: fixed.P(x, y+metrics.Ascent.Ceil())}
	drawer.DrawString(value)
	if bold && ((title && c.data.Font == "Belleza") || (!title && c.data.BodyFont == "Belleza")) {
		drawer.Dot = fixed.P(x+1, y+metrics.Ascent.Ceil())
		drawer.DrawString(value)
	}
	return y + metrics.Height.Ceil(), nil
}

func (c *receiptCanvas) measure(value string, size float64, bold, title bool) (int, error) {
	face, err := c.face(size, bold, title)
	if err != nil {
		return 0, err
	}
	return font.MeasureString(face, value).Ceil(), nil
}

func (c *receiptCanvas) center(y int, value string, size float64, bold, title bool) (int, error) {
	width, err := c.measure(value, size, bold, title)
	if err != nil {
		return y, err
	}
	return c.text(c.margin+(c.contentW-width)/2, y, value, size, bold, title)
}

func (c *receiptCanvas) left(y int, value string, size float64, bold bool) (int, error) {
	return c.text(c.margin, y, value, size, bold, false)
}

func (c *receiptCanvas) indented(y int, value string, size float64) (int, error) {
	return c.text(c.margin+12, y, value, size, false, false)
}

func (c *receiptCanvas) twoCol(y int, left, right string, size float64, bold bool) (int, error) {
	rightWidth, err := c.measure(right, size, bold, false)
	if err != nil {
		return y, err
	}
	if _, err = c.text(c.margin, y, left, size, bold, false); err != nil {
		return y, err
	}
	return c.text(c.margin+c.contentW-rightWidth, y, right, size, bold, false)
}

// itemRow keeps the amount in a protected right-hand column and wraps the
// product name into the remaining space. This prevents long names from being
// drawn underneath the amount on thermal printers.
func (c *receiptCanvas) itemRow(y int, name, amount string, size float64) (int, error) {
	rightWidth, err := c.measure(amount, size, true, false)
	if err != nil {
		return y, err
	}
	nameWidth := c.contentW - rightWidth - 16
	face, err := c.face(size, true, false)
	if err != nil {
		return y, err
	}
	lines := make([]string, 0, 2)
	for _, word := range strings.Fields(name) {
		if len(lines) == 0 {
			lines = append(lines, word)
			continue
		}
		candidate := lines[len(lines)-1] + " " + word
		limit := c.contentW
		if len(lines) == 1 {
			limit = nameWidth
		}
		if font.MeasureString(face, candidate).Ceil() > limit {
			lines = append(lines, word)
		} else {
			lines[len(lines)-1] = candidate
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	for index, line := range lines {
		if index == 0 {
			if _, err = c.text(c.margin, y, line, size, true, false); err != nil {
				return y, err
			}
			if _, err = c.text(c.margin+c.contentW-rightWidth, y, amount, size, true, false); err != nil {
				return y, err
			}
		} else if _, err = c.text(c.margin, y, line, size, true, false); err != nil {
			return y, err
		}
		if index < len(lines)-1 {
			y += 10
		}
	}
	return y, nil
}

func (c *receiptCanvas) centerWrapped(y int, value string, size float64, spacing int) (int, error) {
	for _, paragraph := range strings.Split(value, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := strings.TrimSpace(line + " " + word)
			width, err := c.measure(candidate, size, true, false)
			if err != nil {
				return y, err
			}
			if line != "" && width > c.contentW {
				y, err = c.center(y, line, size, true, false)
				if err != nil {
					return y, err
				}
				y += spacing
				line = word
			} else {
				line = candidate
			}
		}
		if line != "" {
			var err error
			y, err = c.center(y, line, size, true, false)
			if err != nil {
				return y, err
			}
			y += spacing
		}
	}
	return y, nil
}

func (c *receiptCanvas) rule(y int) int {
	for x := c.margin; x < c.margin+c.contentW; x += 8 {
		for dx := 0; dx < 4 && x+dx < c.width; dx++ {
			c.img.Set(x+dx, y, color.Black)
			c.img.Set(x+dx, y+1, color.Black)
		}
	}
	return y + 2
}

func (c *receiptCanvas) dottedBox(y, height int) int {
	for x := c.margin; x < c.margin+c.contentW; x += 8 {
		for dx := 0; dx < 4 && x+dx < c.margin+c.contentW; dx++ {
			c.img.Set(x+dx, y, color.Black)
			c.img.Set(x+dx, y+height, color.Black)
		}
	}
	for yy := y; yy <= y+height; yy += 8 {
		for dx := 0; dx < 4; dx++ {
			if y+height <= c.img.Bounds().Dy() {
				c.img.Set(c.margin, yy+dx, color.Black)
				c.img.Set(c.margin+c.contentW-1, yy+dx, color.Black)
			}
		}
	}
	return y
}

func (c *receiptCanvas) social(y int, item socialFooterItem) (int, error) {
	const iconSize = 32
	iconBytes := socialPlatformIconBytes(item.platform)
	if len(iconBytes) == 0 {
		return c.center(y, item.handle, c.data.BodySize, false, false)
	}
	icon, err := png.Decode(bytes.NewReader(iconBytes))
	if err != nil {
		return y, fmt.Errorf("decode %s icon: %w", item.platform, err)
	}
	textWidth, err := c.measure(item.handle, c.data.BodySize, false, false)
	if err != nil {
		return y, err
	}
	rowWidth := iconSize + 12 + textWidth
	x := c.margin + (c.contentW-rowWidth)/2
	xdraw.CatmullRom.Scale(c.img, image.Rect(x, y, x+iconSize, y+iconSize), icon, icon.Bounds(), xdraw.Over, nil)
	textBottom, err := c.text(x+iconSize+12, y, item.handle, c.data.BodySize, false, false)
	if err != nil {
		return y, err
	}
	if textBottom < y+iconSize {
		textBottom = y + iconSize
	}
	return textBottom, nil
}

// socialFooterRow matches the PDF footer: the label and all platform icons
// share one horizontal row. The first configured handle is rendered below it
// by drawReceipt.
func (c *receiptCanvas) socialFooterRow(y int, items []socialFooterItem) (int, error) {
	const (
		iconSize = 32
		gap      = 16
	)

	label := "Follow us on:"
	labelWidth, err := c.measure(label, c.data.BodySize-1, true, false)
	if err != nil {
		return y, err
	}
	rowWidth := labelWidth + len(items)*iconSize + (len(items)-1)*gap
	startX := c.margin + (c.contentW-rowWidth)/2
	if startX < c.margin {
		startX = c.margin
	}
	if _, err = c.text(startX, y, label, c.data.BodySize-1, true, false); err != nil {
		return y, err
	}

	x := startX + labelWidth
	rowBottom := y
	for _, item := range items {
		iconBytes := socialPlatformIconBytes(item.platform)
		if len(iconBytes) > 0 {
			icon, decodeErr := png.Decode(bytes.NewReader(iconBytes))
			if decodeErr != nil {
				return y, fmt.Errorf("decode %s icon: %w", item.platform, decodeErr)
			}
			xdraw.CatmullRom.Scale(c.img, image.Rect(x, y, x+iconSize, y+iconSize), icon, icon.Bounds(), xdraw.Over, nil)
		}
		x += iconSize + gap
		if y+iconSize > rowBottom {
			rowBottom = y + iconSize
		}
	}
	return rowBottom, nil
}

func (c *receiptCanvas) barcode(y int, receiptNo string) int {
	seed := int64(0)
	for _, char := range receiptNo {
		seed += int64(char)
	}
	rng := rand.New(rand.NewSource(seed))
	x := c.margin
	const height = 68
	widths := []int{2, 3, 4, 6}
	for x < c.margin+c.contentW {
		width := widths[rng.Intn(len(widths))]
		if rng.Float64() > 0.35 {
			draw.Draw(c.img, image.Rect(x, y, minInt(x+width, c.margin+c.contentW), y+height), image.Black, image.Point{}, draw.Src)
		}
		x += width + 2
	}
	return y + height
}

func encodeRasterBands(img *image.RGBA, cutoff bool) []byte {
	rowBytes := (img.Bounds().Dx() + 7) / 8
	var out bytes.Buffer
	out.Write([]byte{esc, '@', esc, 'a', 1, esc, '3', 0})
	for startY := 0; startY < img.Bounds().Dy(); startY += rasterBandHeight {
		height := minInt(rasterBandHeight, img.Bounds().Dy()-startY)
		out.Write([]byte{gs, 'v', '0', 0, byte(rowBytes), byte(rowBytes >> 8), byte(height), byte(height >> 8)})
		for y := startY; y < startY+height; y++ {
			for xByte := 0; xByte < rowBytes; xByte++ {
				var packed byte
				for bit := 0; bit < 8; bit++ {
					x := xByte*8 + bit
					if x >= img.Bounds().Dx() {
						continue
					}
					pixel := img.RGBAAt(x, y)
					if pixel.A > 32 && (int(pixel.R)+int(pixel.G)+int(pixel.B))/3 < 176 {
						packed |= 1 << (7 - bit)
					}
				}
				out.WriteByte(packed)
			}
		}
		out.WriteByte('\n')
	}
	out.Write([]byte{esc, '2', '\n', '\n', '\n'})
	if cutoff {
		// GS V 0 selects a full cut. The previous parameter (3) is not a
		// standard ESC/POS cut mode and is ignored by many printers.
		out.Write([]byte{gs, cutMode, 0})
	}
	return out.Bytes()
}

func validateRasterPayload(payload []byte, width int) error {
	if bytes.Contains(payload, []byte("%PDF")) {
		return fmt.Errorf("PDF data must never be sent to an ESC/POS printer")
	}
	prefix := []byte{esc, '@', esc, 'a', 1, esc, '3', 0}
	if !bytes.HasPrefix(payload, prefix) {
		return fmt.Errorf("missing ESC/POS initialization")
	}
	expectedRowBytes := width / 8
	offset, bands := len(prefix), 0
	for offset+2 <= len(payload) && !(payload[offset] == esc && payload[offset+1] == '2') {
		if offset+8 > len(payload) || payload[offset] != gs || payload[offset+1] != 'v' || payload[offset+2] != '0' || payload[offset+3] != 0 {
			return fmt.Errorf("unexpected command at byte %d", offset)
		}
		rowBytes := int(payload[offset+4]) | int(payload[offset+5])<<8
		height := int(payload[offset+6]) | int(payload[offset+7])<<8
		if rowBytes != expectedRowBytes || height < 1 || height > rasterBandHeight {
			return fmt.Errorf("invalid raster band dimensions %dx%d", rowBytes*8, height)
		}
		offset += 8 + rowBytes*height
		if offset >= len(payload) || payload[offset] != '\n' {
			return fmt.Errorf("truncated raster band")
		}
		offset++
		bands++
	}
	if bands == 0 || offset+2 > len(payload) || payload[offset] != esc || payload[offset+1] != '2' {
		return fmt.Errorf("missing raster data or line-spacing reset")
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
