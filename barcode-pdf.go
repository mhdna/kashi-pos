package main

import (
	"bytes"
	"fmt"
)

// Standard Adobe AFM widths (1/1000 em) for the base-14 fonts, ASCII 32-126.
var helveticaWidths = map[rune]int{
	' ': 278, '!': 278, '"': 355, '#': 556, '$': 556, '%': 889, '&': 667, '\'': 191,
	'(': 333, ')': 333, '*': 389, '+': 584, ',': 278, '-': 333, '.': 278, '/': 278,
	'0': 556, '1': 556, '2': 556, '3': 556, '4': 556, '5': 556, '6': 556, '7': 556, '8': 556, '9': 556,
	':': 278, ';': 278, '<': 584, '=': 584, '>': 584, '?': 556, '@': 1015,
	'A': 667, 'B': 667, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778, 'H': 722, 'I': 278, 'J': 500,
	'K': 667, 'L': 556, 'M': 833, 'N': 722, 'O': 778, 'P': 667, 'Q': 778, 'R': 722, 'S': 667, 'T': 611,
	'U': 722, 'V': 667, 'W': 944, 'X': 667, 'Y': 667, 'Z': 611,
	'[': 278, '\\': 278, ']': 278, '^': 469, '_': 556, '`': 333,
	'a': 556, 'b': 556, 'c': 500, 'd': 556, 'e': 556, 'f': 278, 'g': 556, 'h': 556, 'i': 222, 'j': 222,
	'k': 500, 'l': 222, 'm': 833, 'n': 556, 'o': 556, 'p': 556, 'q': 556, 'r': 333, 's': 500, 't': 278,
	'u': 556, 'v': 500, 'w': 722, 'x': 500, 'y': 500, 'z': 500,
	'{': 334, '|': 260, '}': 334, '~': 584,
}

var helveticaBoldWidths = map[rune]int{
	' ': 278, '!': 333, '"': 474, '#': 556, '$': 556, '%': 889, '&': 722, '\'': 238,
	'(': 333, ')': 333, '*': 389, '+': 584, ',': 278, '-': 333, '.': 278, '/': 278,
	'0': 556, '1': 556, '2': 556, '3': 556, '4': 556, '5': 556, '6': 556, '7': 556, '8': 556, '9': 556,
	':': 333, ';': 333, '<': 584, '=': 584, '>': 584, '?': 611, '@': 975,
	'A': 722, 'B': 722, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778, 'H': 722, 'I': 278, 'J': 556,
	'K': 722, 'L': 611, 'M': 833, 'N': 722, 'O': 778, 'P': 667, 'Q': 778, 'R': 722, 'S': 667, 'T': 611,
	'U': 722, 'V': 667, 'W': 944, 'X': 667, 'Y': 667, 'Z': 611,
	'[': 333, '\\': 278, ']': 333, '^': 584, '_': 556, '`': 333,
	'a': 556, 'b': 611, 'c': 556, 'd': 611, 'e': 556, 'f': 333, 'g': 611, 'h': 611, 'i': 278, 'j': 278,
	'k': 556, 'l': 278, 'm': 889, 'n': 611, 'o': 611, 'p': 611, 'q': 611, 'r': 389, 's': 556, 't': 333,
	'u': 611, 'v': 556, 'w': 778, 'x': 556, 'y': 556, 'z': 500,
	'{': 389, '|': 280, '}': 389, '~': 584,
}

// textWidth returns the width of s set in the given size, using font's AFM width table.
func textWidth(s string, widths map[rune]int, size float64) float64 {
	total := 0
	for _, r := range s {
		w, ok := widths[r]
		if !ok {
			w = 556 // fallback average width
		}
		total += w
	}
	return float64(total) / 1000.0 * size
}

// pdfEscape escapes special characters for a PDF literal string.
func pdfEscape(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pdfWriter accumulates a raw PDF file with manually tracked object offsets.
type pdfWriter struct {
	buf     bytes.Buffer
	offsets []int
}

func (w *pdfWriter) startObj(n int) {
	w.offsets = append(w.offsets, w.buf.Len())
	if len(w.offsets) != n {
		panic("pdf object numbering out of sync")
	}
	fmt.Fprintf(&w.buf, "%d 0 obj\n", n)
}

func (w *pdfWriter) endObj() {
	w.buf.WriteString("endobj\n")
}

// buildLabelPDF renders the price/name/code/barcode label matching the reference layout,
// returning the complete PDF file bytes.
func buildLabelPDF(priceStr, name, code, barcodeDigits, modules string) []byte {
	const (
		pageW = 220.0 // narrower label, widened slightly for barcode quiet zone
		pageH = 148.0 // ~2.06in
	)

	priceText := priceStr + " $"
	priceSize := 26.0
	nameSize := 19.0
	codeSize := 12.0
	digitsSize := 11.0

	priceW := textWidth(priceText, helveticaBoldWidths, priceSize)
	nameW := textWidth(name, helveticaWidths, nameSize)
	codeW := textWidth(code, helveticaWidths, codeSize)

	priceX := (pageW - priceW) / 2
	nameX := (pageW - nameW) / 2
	codeX := (pageW - codeW) / 2

	// Barcode geometry: scale modules to fit within a comfortable width.
	moduleW := 1.6
	barcodeW := float64(len(modules)) * moduleW
	barcodeX := (pageW - barcodeW) / 2
	barcodeY := 38.0
	barcodeH := 28.0 // 2x taller data bars

	// Guard bars (start/center/end) run taller than the digit bars, extending
	// further down, matching how real EAN-13 barcodes are printed.
	const (
		startGuardLen  = 3
		digitBlockLen  = 42 // 6 digits * 7 modules
		centerGuardLen = 5
	)
	leftDigitsEnd := startGuardLen + digitBlockLen   // 45
	centerGuardEnd := leftDigitsEnd + centerGuardLen // 50
	rightDigitsEnd := centerGuardEnd + digitBlockLen // 92
	isGuard := func(i int) bool {
		return i < startGuardLen ||
			(i >= leftDigitsEnd && i < centerGuardEnd) ||
			i >= rightDigitsEnd
	}
	guardExtra := 8.0
	guardY := barcodeY - guardExtra
	guardH := barcodeH + guardExtra

	var content bytes.Buffer
	content.WriteString("q\n0 0 0 rg\n0 0 0 RG\n")

	// Price (bold)
	fmt.Fprintf(&content, "BT /FB %.2f Tf %.2f %.2f Td (%s) Tj ET\n",
		priceSize, priceX, 118.0, pdfEscape(priceText))

	// Underline below price, drawn twice as wide as the price text and centered.
	lineY := 112.0
	lineW := priceW * 2
	if lineW > pageW-20 {
		lineW = pageW - 20
	}
	lineX := (pageW - lineW) / 2
	fmt.Fprintf(&content, "%.2f w %.2f %.2f m %.2f %.2f l S\n",
		1.2, lineX, lineY, lineX+lineW, lineY)

	// Name
	fmt.Fprintf(&content, "BT /F1 %.2f Tf %.2f %.2f Td (%s) Tj ET\n",
		nameSize, nameX, 96.0, pdfEscape(name))

	// Code
	fmt.Fprintf(&content, "BT /F1 %.2f Tf %.2f %.2f Td (%s) Tj ET\n",
		codeSize, codeX, 76.0, pdfEscape(code))

	// Barcode bars: draw a filled rect per '1' run; guard bars taller than digit bars.
	x := barcodeX
	for i := 0; i < len(modules); i++ {
		if modules[i] == '1' {
			if isGuard(i) {
				fmt.Fprintf(&content, "%.3f %.2f %.3f %.2f re f\n", x, guardY, moduleW, guardH)
			} else {
				fmt.Fprintf(&content, "%.3f %.2f %.3f %.2f re f\n", x, barcodeY, moduleW, barcodeH)
			}
		}
		x += moduleW
	}

	// Digits under barcode, stretched via character spacing so the digit string
	// spans exactly the same width as the barcode itself.
	digitsSpacingBase := textWidth(barcodeDigits, helveticaWidths, digitsSize)
	digitsTc := 0.0
	if n := len(barcodeDigits); n > 1 {
		digitsTc = (barcodeW - digitsSpacingBase) / float64(n-1)
	}
	fmt.Fprintf(&content, "BT /F1 %.2f Tf %.2f Tc %.2f %.2f Td (%s) Tj ET\n",
		digitsSize, digitsTc, barcodeX, guardY-12.0, pdfEscape(barcodeDigits))

	content.WriteString("Q\n")

	w := &pdfWriter{}
	w.buf.WriteString("%PDF-1.4\n")

	// Obj 1: Catalog
	w.startObj(1)
	w.buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	w.endObj()

	// Obj 2: Pages
	w.startObj(2)
	w.buf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	w.endObj()

	// Obj 3: Page
	w.startObj(3)
	fmt.Fprintf(&w.buf, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 4 0 R /FB 5 0 R >> >> /Contents 6 0 R >>\n", pageW, pageH)
	w.endObj()

	// Obj 4: Helvetica
	w.startObj(4)
	w.buf.WriteString("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n")
	w.endObj()

	// Obj 5: Helvetica-Bold
	w.startObj(5)
	w.buf.WriteString("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\n")
	w.endObj()

	// Obj 6: Content stream
	w.startObj(6)
	fmt.Fprintf(&w.buf, "<< /Length %d >>\nstream\n%s\nendstream\n", content.Len(), content.String())
	w.endObj()

	xrefStart := w.buf.Len()
	fmt.Fprintf(&w.buf, "xref\n0 %d\n", len(w.offsets)+1)
	w.buf.WriteString("0000000000 65535 f \n")
	for _, off := range w.offsets {
		fmt.Fprintf(&w.buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&w.buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(w.offsets)+1, xrefStart)

	return w.buf.Bytes()
}
