// Generates a mock 80mm thermal-printer style receipt (grayscale)
// for a fictional clothing brand, using github.com/go-pdf/fpdf.
package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-pdf/fpdf"
)

// ---------- Thermal roll geometry (80mm paper, mm units) ----------
const (
	pageW    = 80.0
	margin   = 4.0
	pageH    = 260.0 // tall enough to avoid clipping; trimmed at the end
	contentW = pageW - 2*margin
)

// ---------- Font sizes ----------
const (
	fontSizeTitle    = 21.0
	fontSizeSubtitle = 12.5
	fontSizeBody     = 11.0
	fontSizeMeta     = 11.0
	fontSizeSummary  = 11.0
	fontSizeTotal    = 13.0
	fontSizePolicy   = 9.5
	fontSizeThankyou = 9.5
	fontSizeFooter   = 9.0
)

const (
	fontFamily = "Courier"
	styleReg   = ""  // regular weight — used sparingly, thin on thermal paper
	styleBold  = "B" // bold weight — default for almost everything, prints crisp
)

// ---------- Fictional store data ----------
const (
	storeName    = "Demo Store"
	storeTagline = "Ultimate Shopping Destination"
	branch       = "Branch: Main Branch"
	phone        = "Tel: +961 70 000 000"
	salesperson  = "Sold by: Sarah M."
	cashierID    = "Register #1"
	receiptNo    = "INV-2026-00001"
	currency     = "$"
	taxRate      = 0.00 // 11% VAT
	lbpRate      = 90000.0
)

type item struct {
	name    string
	code    string
	qty     float64
	price   float64
	discPct float64
}

var items = []item{
	{"Kids Boy Pants - White", "EOC5KBP12", 1, 12.00, 0},
	{"Lady Shorts - Khaki", "EF6LSHO15", 1, 15.00, 15},
	{"Men Jeans", "COC5MJE25", 2, 25.00, 10},
	{"Leather Pants - Brown", "COC5LP22", 1, 22.00, 0},
	{"Cotton Socks (3-Pack)", "COC5CSO8", 1, 8.00, 0},
}

func lineTotal(it item) (total, discAmt float64) {
	gross := it.qty * it.price
	discAmt = gross * (it.discPct / 100.0)
	total = gross - discAmt
	return
}

// Receipt wraps the pdf and a running y cursor.
type Receipt struct {
	pdf *fpdf.Fpdf
	y   float64
}

func NewReceipt() *Receipt {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "",
		Size:           fpdf.SizeType{Wd: pageW, Ht: pageH},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(0, 0, 0)
	return &Receipt{pdf: pdf, y: 8}
}

func (r *Receipt) down(mm float64) { r.y += mm }

// align: "L", "C", "R". style is "" or "B".
func (r *Receipt) text(x float64, s string, style string, size float64, align string) {
	r.pdf.SetFont(fontFamily, style, size)
	w := contentW
	startX := x
	if align == "C" {
		startX = margin
		w = contentW
	} else if align == "R" {
		startX = margin
		w = contentW
	}
	r.pdf.SetXY(startX, r.y)
	r.pdf.CellFormat(w, 4, s, "", 0, align, false, 0, "")
}

func (r *Receipt) center(s, style string, size float64) {
	r.text(margin, s, style, size, "C")
}

func (r *Receipt) twoCol(left, right, style string, size float64) {
	r.pdf.SetFont(fontFamily, style, size)
	r.pdf.SetXY(margin, r.y)
	r.pdf.CellFormat(contentW/2, 4, left, "", 0, "L", false, 0, "")
	r.pdf.SetXY(margin+contentW/2, r.y)
	r.pdf.CellFormat(contentW/2, 4, right, "", 0, "R", false, 0, "")
}

// Thermal printers blur thin strokes and light dot patterns, so rules are
// drawn thicker and with a wider dash gap than you'd use on plain paper.
func (r *Receipt) dashedRule() {
	r.pdf.SetDashPattern([]float64{1.1, 1.0}, 0)
	r.pdf.SetLineWidth(0.35)
	r.pdf.Line(margin, r.y, pageW-margin, r.y)
	r.pdf.SetDashPattern([]float64{}, 0)
}

func (r *Receipt) solidRule() {
	r.pdf.SetLineWidth(0.6)
	r.pdf.Line(margin, r.y, pageW-margin, r.y)
}

// simple greedy word-wrap using cell string width
func (r *Receipt) wrapText(s, style string, size, maxWidth float64) []string {
	r.pdf.SetFont(fontFamily, style, size)
	words := []rune{}
	_ = words
	var lines []string
	var cur string
	for _, w := range splitWords(s) {
		trial := cur
		if trial != "" {
			trial += " "
		}
		trial += w
		if r.pdf.GetStringWidth(trial) <= maxWidth {
			cur = trial
		} else {
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func splitWords(s string) []string {
	var words []string
	start := 0
	for i, c := range s {
		if c == ' ' {
			if i > start {
				words = append(words, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		words = append(words, s[start:])
	}
	return words
}

// barcode draws a fake barcode derived deterministically from a code string.
func (r *Receipt) barcode(code string, height float64) {
	seed := int64(0)
	for _, c := range code {
		seed += int64(c)
	}
	rnd := rand.New(rand.NewSource(seed))
	widths := []float64{0.35, 0.55, 0.8, 1.05}
	cx := margin
	top := r.y
	for cx < margin+contentW {
		w := widths[rnd.Intn(len(widths))]
		if rnd.Float64() > 0.35 {
			r.pdf.SetLineWidth(w)
			r.pdf.Line(cx, top, cx, top+height)
		}
		cx += w + 0.3
	}
	r.y += height
}

func (r *Receipt) Build() {
	// Bold is the default weight throughout — thermal heads blot out thin
	// strokes, so regular weight is only used for the least-important lines
	// (item codes / qty breakdowns), and everything is sized up a notch.

	// ---- Header ----
	r.center(storeName, styleBold, fontSizeTitle)
	r.down(8.5)
	r.center(storeTagline, styleBold, fontSizeSubtitle)
	r.down(5)
	r.center(branch, styleBold, fontSizeSubtitle)
	r.down(4.5)
	r.center(phone, styleBold, fontSizeSubtitle)
	r.down(5.5)

	r.dashedRule()
	r.down(4.5)

	// ---- Meta info ----
	r.twoCol(fmt.Sprintf("Receipt: %s", receiptNo), "", styleBold, fontSizeMeta)
	r.down(4.5)
	now := time.Now()
	r.twoCol("Date: "+now.Format("02/01/2006"), "Time: "+now.Format("15:04"), styleBold, fontSizeMeta)
	r.down(4.5)
	r.twoCol(salesperson, cashierID, styleBold, fontSizeBody)
	r.down(5.5)

	r.dashedRule()
	r.down(4.5)

	// ---- Items header ----
	r.twoCol("ITEM", "TOTAL", styleBold, fontSizeMeta)
	r.down(3.8)
	r.dashedRule()
	r.down(4.2)

	var subtotal, totalDiscount float64

	for _, it := range items {
		total, discAmt := lineTotal(it)
		subtotal += it.qty * it.price
		totalDiscount += discAmt

		wrapped := r.wrapText(it.name, styleBold, fontSizeBody, contentW-24)
		first := it.name
		if len(wrapped) > 0 {
			first = wrapped[0]
		}
		r.twoCol(first, fmt.Sprintf("%s%.2f", currency, total), styleBold, fontSizeMeta)
		r.down(3.9)
		for _, extra := range wrapped[1:] {
			r.text(margin, extra, styleBold, fontSizeBody, "L")
			r.down(3.9)
		}

		r.text(margin+2, "Code: "+it.code, styleReg, fontSizeBody, "L")
		r.down(3.5)

		qtyLine := fmt.Sprintf("%.0f x %s%.2f", it.qty, currency, it.price)
		if it.discPct > 0 {
			qtyLine += fmt.Sprintf("   (-%.0f%% = -%s%.2f)", it.discPct, currency, discAmt)
		}
		r.text(margin+2, qtyLine, styleReg, fontSizeBody, "L")
		r.down(4.8)
	}

	r.dashedRule()
	r.down(4.5)

	taxable := subtotal - totalDiscount
	tax := taxable * taxRate
	grandTotal := taxable + tax

	r.twoCol("Subtotal", fmt.Sprintf("%s%.2f", currency, subtotal), styleBold, fontSizeSummary)
	r.down(4.4)
	r.twoCol("Discount", fmt.Sprintf("-%s%.2f", currency, totalDiscount), styleBold, fontSizeSummary)
	r.down(4.4)
	r.twoCol(fmt.Sprintf("VAT (%.0f%%)", taxRate*100), fmt.Sprintf("%s%.2f", currency, tax), styleBold, fontSizeSummary)
	r.down(5)

	r.solidRule()
	r.down(5.8)

	r.twoCol("TOTAL DUE", fmt.Sprintf("%s%.2f", currency, grandTotal), styleBold, fontSizeTotal)
	r.down(5.5)
	r.twoCol("LBP TOTAL", fmt.Sprintf("%.0f L.L.", grandTotal*lbpRate), styleBold, fontSizeSummary)
	r.down(7)

	r.dashedRule()
	r.down(4.5)

	// Payment method
	r.twoCol("Payment Method:", "Cash", styleBold, fontSizeMeta)
	r.down(4.4)
	qtySum := 0.0
	for _, it := range items {
		qtySum += it.qty
	}
	r.twoCol("Items Purchased:", fmt.Sprintf("%.0f", qtySum), styleBold, fontSizeMeta)
	r.down(5.5)

	r.dashedRule()
	r.down(5)

	// ---- Return policy ----
	r.center("RETURN & EXCHANGE POLICY", styleBold, fontSizeSummary)
	r.down(4.6)
	policyLines := []string{
		"Returns accepted within 3 DAYS of ",
		"purchase date, with this receipt as ",
		"proof of purchase. Items must be ",
		"unworn, unwashed, and have ",
		"all original tags attached.",
	}
	for _, pl := range policyLines {
		r.center(pl, styleBold, fontSizePolicy)
		r.down(3.7)
	}

	r.down(1.5)
	r.dashedRule()
	r.down(6.5)

	// ---- Footer ----
	r.center("Thank you for shopping with us!", styleBold, fontSizeThankyou)
	r.down(4.8)
	r.center("Follow us @demostore", styleBold, fontSizeFooter)
	r.down(4.2)
	r.center("www.example.com", styleBold, fontSizeFooter)
	r.down(6.5)

	r.barcode(receiptNo, 9)
	r.down(3.2)
	r.center(receiptNo, styleBold, fontSizeMeta)
	r.down(6)
}

func main() {
	r := NewReceipt()
	r.Build()
	if err := r.pdf.OutputFileAndClose("clothing_receipt_go.pdf"); err != nil {
		panic(err)
	}
	fmt.Println("done, final y (mm):", r.y)
}
