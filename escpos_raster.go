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
	style    receiptStyle
	layout   receiptLayoutSpec
	img      *image.RGBA
	width    int
	margin   int
	contentW int
	faces    map[string]font.Face
}

func buildESCPOSRasterReceipt(data printReceiptData) ([]byte, error) {
	cropped, err := renderReceiptRaster(data)
	if err != nil {
		return nil, err
	}
	payload := encodeRasterBands(cropped, data.Cutoff)
	if err := validateRasterPayload(payload, cropped.Bounds().Dx()); err != nil {
		return nil, fmt.Errorf("unsafe printer payload rejected: %w", err)
	}
	return payload, nil
}

func renderReceiptRaster(data printReceiptData) (*image.RGBA, error) {
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
	// The canvas is allocated up front, so this is an upper estimate; the
	// drawn height is checked below. Themed layouts can take more room per
	// item than the classic one, and narrow paper wraps names further.
	estimatedHeight := 1900 + len(data.Items)*150 + len(data.SocialPlatforms)*60 + len(data.ReturnPolicy)*2
	if data.ShiftReport != nil {
		estimatedHeight = 2400 + len(data.ShiftReport.Payments)*60
	}
	if estimatedHeight > rasterMaxHeight {
		if data.ShiftReport != nil || len(data.Items) > 120 {
			return nil, fmt.Errorf("receipt is too long to render safely")
		}
		estimatedHeight = rasterMaxHeight
	}

	theme, layout := resolveReceiptTheme(defaultReceiptTheme, defaultReceiptLayout)
	if data.ShiftReport == nil {
		theme, layout = resolveReceiptTheme(data.Theme, data.Layout)
	}
	c := &receiptCanvas{
		data:     data,
		style:    theme.Style,
		layout:   layout,
		img:      image.NewRGBA(image.Rect(0, 0, width, estimatedHeight)),
		width:    width,
		margin:   24,
		contentW: width - 48,
		faces:    make(map[string]font.Face),
	}
	draw.Draw(c.img, c.img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	var y int
	var err error
	if data.ShiftReport != nil {
		y, err = c.drawShiftReport()
	} else {
		y, err = c.drawReceipt()
	}
	if err != nil {
		return nil, err
	}
	if y <= 0 || y > c.img.Bounds().Dy() {
		return nil, fmt.Errorf("receipt raster height is invalid: %d", y)
	}

	top := firstRasterInkRow(c.img, y)
	cropped := image.NewRGBA(image.Rect(0, 0, width, y-top))
	draw.Draw(cropped, cropped.Bounds(), c.img, image.Point{Y: top}, draw.Src)
	return cropped, nil
}

func firstRasterInkRow(img *image.RGBA, height int) int {
	for y := 0; y < height; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			pixel := img.RGBAAt(x, y)
			if pixel.A > 32 && (int(pixel.R)+int(pixel.G)+int(pixel.B))/3 < 176 {
				return y
			}
		}
	}
	return 0
}

// gap is a vertical space of n pixels plus the theme's extra breathing room.
func (c *receiptCanvas) gap(n int) int { return n + c.style.Gap }

func (c *receiptCanvas) drawReceipt() (int, error) {
	y := 0
	var err error
	store, branch := c.data.StoreName, c.data.Branch
	if c.style.UpperStore {
		store, branch = strings.ToUpper(store), strings.ToUpper(branch)
	}
	if y, err = c.center(y, store, c.data.TitleSize*c.titleFactor(), true, true); err != nil {
		return 0, err
	}
	y += c.gap(4)
	branchPhone := c.data.BranchPhone
	if branchPhone == "" {
		branchPhone = c.data.Phone
	}
	branchSize := c.data.TitleSize - 8
	phoneSize := c.data.TitleSize - 8
	if branchSize <= 0 {
		branchSize = c.data.BodySize
	}
	if phoneSize <= 0 {
		phoneSize = c.data.BodySize - 1
	}
	if branch != "" {
		y, _ = c.center(y+2, branch, branchSize, false, true)
		y += c.gap(10)
	}
	if branchPhone != "" {
		y, _ = c.center(y, branchPhone, phoneSize, false, true)
		y += c.gap(12)
	}
	// Receipt description temporarily disabled.
	// if c.data.Description != "" {
	// 	y, _ = c.centerWrappedNormal(y, c.data.Description, c.data.BodySize-1, 8)
	// }
	y, _ = c.center(y+c.gap(8), receiptInvoiceLabel(c.data.ReceiptNo)+": "+c.data.ReceiptNo, c.data.BodySize-1, true, false)
	y += c.gap(8)
	if strings.TrimSpace(c.data.Website) != "" {
		y, _ = c.center(y, strings.TrimSpace(c.data.Website), c.data.BodySize-1.5, true, false)
		y += c.gap(8)
	}
	y = c.ruleLine(y) + c.gap(10)
	y = c.drawMeta(y)
	if strings.TrimSpace(c.data.Notes) != "" {
		y += c.gap(6)
		y, _ = c.left(y, "Note: "+strings.TrimSpace(c.data.Notes), c.data.BodySize-1, false)
		y += c.gap(8)
	}
	y = c.ruleLine(y) + c.gap(10)

	y, subtotal, discountTotal := c.layout.draw(c, y)

	y = c.ruleLine(y) + c.gap(10)
	grandTotal := float64(c.data.GrandTotalCents) / 100
	// See buildESCPosReceipt: the grand total already carries the cash-grid
	// snap, so tax has to come from the invoice rather than be inferred from
	// the difference, and the leftover prints as rounding.
	tax := float64(c.data.TaxCents) / 100
	rounding := grandTotal - (subtotal - discountTotal) - tax
	rows := [][2]string{{"Subtotal", fmt.Sprintf("$%.2f", subtotal)}}
	if discountTotal > 0 {
		rows = append(rows, [2]string{"Discount", fmt.Sprintf("-$%.2f", discountTotal)})
	}
	if math.Abs(rounding) > 0.005 {
		rows = append(rows, [2]string{"Rounding", fmt.Sprintf("$%.2f", rounding)})
	}
	if c.style.ItemCount {
		var units float64
		for _, item := range c.data.Items {
			units += item.Qty
		}
		rows = append(rows, [2]string{"Items", fmt.Sprintf("%.0f", units)})
	}
	if c.style.Savings && discountTotal > 0 {
		rows = append(rows, [2]string{"You saved", fmt.Sprintf("$%.2f", discountTotal)})
	}
	for _, row := range rows {
		y, _ = c.twoCol(y, row[0], row[1], 11, true)
		y += c.gap(7)
	}
	y = c.drawTotal(y, grandTotal)
	y += c.gap(6)

	if len(c.data.Payments) > 0 {
		y, _ = c.center(y, "PAYMENT DETAILS", c.data.BodySize-1, true, false)
		y += c.gap(8)
		for _, payment := range c.data.Payments {
			label := payment.AccountName
			if label == "" {
				label = "Cash"
			}
			if payment.EntryType == "change" {
				label = "Change - " + label
			} else {
				label = "Received - " + label
			}
			y, _ = c.twoCol(y, label, formatReceiptPayment(payment, c.data.RoundingMode), 10, true)
			y += c.gap(7)
		}
	}

	if len(c.data.Payments) > 0 {
		y = c.ruleLine(y) + c.gap(10)
	}
	y, _ = c.center(y, "EXCHANGE POLICY", c.data.BodySize-1.5, true, false)
	y += c.gap(8)
	policy := c.data.ReturnPolicy
	if policy == "" {
		policy = "Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt."
	}
	y, _ = c.centerWrappedNormal(y, policy, c.data.BodySize-2.5, 8)
	y = c.ruleLine(y+c.gap(6)) + c.gap(10)

	socials := socialFooterItems(c.data.SocialPlatforms, c.data.SocialHandles)
	if len(socials) > 0 {
		var rowBottom int
		if rowBottom, err = c.socialFooterRow(y, socials); err != nil {
			return 0, err
		}
		y = rowBottom + c.gap(10)
		// Match the PDF receipt: show one centered handle beneath the icon row.
		if y, err = c.center(y, socials[0].handle, c.data.BodySize-1.5, false, false); err != nil {
			return 0, err
		}
		y += c.gap(8)
	}
	y += c.gap(10)
	thanks := c.style.Thanks
	if thanks == "" {
		thanks = "Thank you for shopping with us!"
	}
	y, _ = c.center(y, thanks, c.data.BodySize-1, true, false)
	y += 56
	// Printer barcode temporarily disabled.
	// y = c.barcode(y, c.data.ReceiptNo) + 24
	y += 24
	return y, nil
}

// fontRole is which of a theme's fonts a piece of text uses.
type fontRole int

const (
	roleBody fontRole = iota
	roleBodyBold
	roleTitle
	roleTitleBold
	roleName // item names
)

func fontRoleFor(bold, title bool) fontRole {
	switch {
	case title && bold:
		return roleTitleBold
	case title:
		return roleTitle
	case bold:
		return roleBodyBold
	}
	return roleBody
}

func (c *receiptCanvas) face(size float64, bold, title bool) (font.Face, error) {
	return c.faceRole(size, fontRoleFor(bold, title))
}

// titleFactor and bodyFactor are the theme's point-size scales; a zero value
// (the classic style) means 1.
func (c *receiptCanvas) titleFactor() float64 {
	if c.style.TitleScale > 0 {
		return c.style.TitleScale
	}
	return 1
}

func (c *receiptCanvas) bodyFactor() float64 {
	if c.style.BodyScale > 0 {
		return c.style.BodyScale
	}
	return 1
}

func (c *receiptCanvas) faceRole(size float64, role fontRole) (font.Face, error) {
	key := fmt.Sprintf("%.2f:%d", size, role)
	if face := c.faces[key]; face != nil {
		return face, nil
	}
	title := role == roleTitle || role == roleTitleBold
	bold := role == roleBodyBold || role == roleTitleBold || role == roleName
	scale := c.bodyFactor()
	var fontData []byte
	if fonts := c.style.Fonts; fonts != nil {
		switch role {
		case roleTitle:
			fontData = fonts.Title
		case roleTitleBold:
			fontData = fonts.TitleBold
		case roleBodyBold:
			fontData = fonts.BodyBold
		case roleName:
			fontData = fonts.Name
			if fontData == nil {
				fontData = fonts.BodyBold
			}
		default:
			fontData = fonts.Body
		}
		if title {
			scale = c.titleFactor()
		}
	} else {
		fontData = gomono.TTF
		if title {
			fontData = goregular.TTF
			if bold {
				fontData = gobold.TTF
			}
		} else if bold {
			fontData = gobold.TTF
		}
		if (title && c.data.Font == "Belleza") || (!title && c.data.BodyFont == "Belleza") {
			fontData = bellezaFont
		} else if !title && (c.data.BodyFont == "CascadiaMono" || c.data.BodyFont == "Helvetica" || c.data.BodyFont == "Courier" || c.data.BodyFont == "Printer Font A") {
			fontData = cascadiaMonoRegular
			if bold {
				fontData = cascadiaMonoBold
			}
		}
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse receipt font: %w", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size * scale * 2.6, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("create receipt font: %w", err)
	}
	c.faces[key] = face
	return face, nil
}

func (c *receiptCanvas) text(x, y int, value string, size float64, bold, title bool) (int, error) {
	return c.textRole(x, y, value, size, fontRoleFor(bold, title), image.Black)
}

func (c *receiptCanvas) textRole(x, y int, value string, size float64, role fontRole, ink image.Image) (int, error) {
	face, err := c.faceRole(size, role)
	if err != nil {
		return y, err
	}
	metrics := face.Metrics()
	drawer := font.Drawer{Dst: c.img, Src: ink, Face: face, Dot: fixed.P(x, y+metrics.Ascent.Ceil())}
	drawer.DrawString(value)
	title := role == roleTitle || role == roleTitleBold
	bold := role == roleBodyBold || role == roleTitleBold || role == roleName
	if c.style.Fonts == nil && bold && ((title && c.data.Font == "Belleza") || (!title && c.data.BodyFont == "Belleza")) {
		drawer.Dot = fixed.P(x+1, y+metrics.Ascent.Ceil())
		drawer.DrawString(value)
	}
	return y + metrics.Height.Ceil(), nil
}

func (c *receiptCanvas) measure(value string, size float64, bold, title bool) (int, error) {
	return c.measureRole(value, size, fontRoleFor(bold, title))
}

func (c *receiptCanvas) measureRole(value string, size float64, role fontRole) (int, error) {
	face, err := c.faceRole(size, role)
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
	lines := wrapRasterItemName(face, name, nameWidth, c.contentW)
	lineY := y
	bottom := y
	for index, line := range lines {
		lineBottom, drawErr := c.text(c.margin, lineY, line, size, true, false)
		if drawErr != nil {
			return y, drawErr
		}
		if index == 0 {
			amountBottom, amountErr := c.text(c.margin+c.contentW-rightWidth, lineY, amount, size, true, false)
			if amountErr != nil {
				return y, amountErr
			}
			if amountBottom > lineBottom {
				lineBottom = amountBottom
			}
		}
		bottom = lineBottom
		if index < len(lines)-1 {
			lineY = bottom + 2
		}
	}
	return bottom, nil
}

func wrapRasterItemName(face font.Face, name string, firstWidth, fullWidth int) []string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return []string{""}
	}
	if firstWidth < 1 {
		firstWidth = 1
	}
	if fullWidth < 1 {
		fullWidth = 1
	}

	lines := []string{""}
	for _, word := range words {
		lineIndex := len(lines) - 1
		limit := fullWidth
		if lineIndex == 0 {
			limit = firstWidth
		}
		candidate := word
		if lines[lineIndex] != "" {
			candidate = lines[lineIndex] + " " + word
		}
		if font.MeasureString(face, candidate).Ceil() <= limit {
			lines[lineIndex] = candidate
			continue
		}

		if lines[lineIndex] != "" {
			lines = append(lines, "")
			lineIndex++
			limit = fullWidth
		}
		for _, character := range word {
			candidate = lines[lineIndex] + string(character)
			if lines[lineIndex] != "" && font.MeasureString(face, candidate).Ceil() > limit {
				lines = append(lines, string(character))
				lineIndex++
				limit = fullWidth
			} else {
				lines[lineIndex] = candidate
			}
		}
	}
	return lines
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

func (c *receiptCanvas) centerWrappedNormal(y int, value string, size float64, spacing int) (int, error) {
	for _, paragraph := range strings.Split(value, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := strings.TrimSpace(line + " " + word)
			width, err := c.measure(candidate, size, false, false)
			if err != nil {
				return y, err
			}
			if line != "" && width > c.contentW {
				y, err = c.center(y, line, size, false, false)
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
			y, err = c.center(y, line, size, false, false)
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
		// labelGap keeps the first icon off the colon.
		labelGap = 10
	)

	label := "Follow us on:"
	labelWidth, err := c.measure(label, c.data.BodySize-1, true, false)
	if err != nil {
		return y, err
	}
	rowWidth := labelWidth + labelGap + len(items)*iconSize + (len(items)-1)*gap
	startX := c.margin + (c.contentW-rowWidth)/2
	if startX < c.margin {
		startX = c.margin
	}
	if _, err = c.text(startX, y, label, c.data.BodySize-1, true, false); err != nil {
		return y, err
	}

	x := startX + labelWidth + labelGap
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
