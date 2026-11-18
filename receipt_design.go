package main

import (
	"image"
	"slices"
	"time"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
)

var blackInk image.Image = image.Black

func measureFace(face font.Face, s string) int { return font.MeasureString(face, s).Ceil() }

// The receipt's design: every line central's Receipt Designer can change,
// each an element with its own words, capitals, size, weight, alignment and
// whether it prints (receiptTexts.Elements, saved in settings.receipt_texts).
// Empty is the default for each, so a till with nothing set prints as before.

// receiptElement is how one line of the receipt is changed.
type receiptElement struct {
	// Text replaces the line's words ({percent} and {currency} are filled in
	// where the line has them). Empty keeps the default.
	Text string `json:"text,omitempty"`
	// Case: "" as written, "upper", "lower" or "title" (First Letters).
	Case string `json:"case,omitempty"`
	// Size is added to the line's size, in the receipt's points.
	Size float64 `json:"size,omitempty"`
	// Bold, when set, makes the line bold or not.
	Bold *bool `json:"bold,omitempty"`
	// Align: "" the look's own, "left", "center" or "right".
	Align string `json:"align,omitempty"`
	// Hide leaves the line out.
	Hide bool `json:"hide,omitempty"`
}

// What an element can change.
const (
	canText  = 1 << iota // its words
	canCase              // capitals
	canSize              // size
	canBold              // weight
	canAlign             // alignment
	canHide              // leave it out
)

const canAll = canText | canCase | canSize | canBold | canAlign | canHide

// receiptElementInfo describes one element for the designer.
type receiptElementInfo struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Label string `json:"label"`
	// Default is the words the till prints unless changed; empty for a line
	// that prints the shop's own data (its name, phone, policy).
	Default string `json:"default"`
	Can     int    `json:"can"`
}

// receiptGroupInfo is one clickable part of the receipt: the elements it holds.
type receiptGroupInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Section, when set, is the receiptSections entry that leaves the whole
	// part out.
	Section string `json:"section,omitempty"`
}

var receiptGroups = []receiptGroupInfo{
	{Key: "header", Label: "Shop and invoice number"},
	{Key: "meta", Label: "Customer, salesperson and date"},
	{Key: "columns", Label: "Items and column titles"},
	{Key: "totals", Label: "Totals"},
	{Key: "payments", Label: "Payment details", Section: receiptSectionPayments},
	{Key: "policy", Label: "Exchange policy", Section: receiptSectionPolicy},
	{Key: "referral", Label: "Referral code", Section: receiptSectionReferral},
	{Key: "socials", Label: "Socials", Section: receiptSectionSocials},
	{Key: "thanks", Label: "Thank-you line", Section: receiptSectionThanks},
}

var receiptElements = []receiptElementInfo{
	{Key: "store_name", Group: "header", Label: "Store name", Can: canAll},
	{Key: "branch_name", Group: "header", Label: "Branch name", Can: canAll},
	{Key: "phone", Group: "header", Label: "Phone", Can: canAll},
	{Key: "invoice_number", Group: "header", Label: "Invoice number line", Can: canSize | canBold | canAlign | canHide},
	{Key: "invoice_label_sale", Group: "header", Label: "Sale invoice title", Default: "Sales Invoice", Can: canText | canCase},
	{Key: "invoice_label_exchange", Group: "header", Label: "Exchange invoice title", Default: "Exchange Invoice", Can: canText | canCase},
	{Key: "invoice_label_return", Group: "header", Label: "Return invoice title", Default: "Return Invoice", Can: canText | canCase},
	{Key: "website", Group: "header", Label: "Website", Can: canAll},

	{Key: "meta_customer", Group: "meta", Label: "Customer", Default: "Customer", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "meta_salesperson", Group: "meta", Label: "Salesperson", Default: "Salesperson", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "meta_date", Group: "meta", Label: "Date", Default: "Date", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "note_label", Group: "meta", Label: "Note", Default: "Note", Can: canText | canCase},

	{Key: "col_item", Group: "columns", Label: "Item", Default: "ITEM", Can: canText | canCase},
	{Key: "col_qty", Group: "columns", Label: "Quantity", Default: "QTY", Can: canText | canCase},
	{Key: "col_disc", Group: "columns", Label: "Discount", Default: "DISC", Can: canText | canCase},
	{Key: "col_price", Group: "columns", Label: "Price", Default: "PRICE", Can: canText | canCase},
	{Key: "col_total", Group: "columns", Label: "Total", Default: "TOTAL", Can: canText | canCase},
	{Key: "col_amount", Group: "columns", Label: "Amount", Default: "AMOUNT", Can: canText | canCase},

	{Key: "subtotal", Group: "totals", Label: "Subtotal", Default: "Subtotal", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "discount", Group: "totals", Label: "Discount", Default: "Discount", Can: canText | canCase | canSize | canBold},
	{Key: "rounding", Group: "totals", Label: "Rounding", Default: "Rounding", Can: canText | canCase | canSize | canBold},
	{Key: "items_count", Group: "totals", Label: "Items count", Default: "Items", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "you_saved", Group: "totals", Label: "You saved", Default: "You saved", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "total", Group: "totals", Label: "Total", Default: "TOTAL", Can: canText | canCase},
	{Key: "equivalent", Group: "totals", Label: "Other currency total", Default: "{currency} equivalent", Can: canText | canCase | canSize | canBold | canHide},

	{Key: "payments_title", Group: "payments", Label: "Title", Default: defaultPaymentsTitle, Can: canAll},
	{Key: "received", Group: "payments", Label: "Paid line", Default: "Received", Can: canText | canCase | canSize | canBold},
	{Key: "change", Group: "payments", Label: "Change line", Default: "Change", Can: canText | canCase | canSize | canBold},

	{Key: "policy_title", Group: "policy", Label: "Title", Default: defaultPolicyTitle, Can: canAll},
	{Key: "policy_text", Group: "policy", Label: "Policy", Default: defaultReturnPolicy, Can: canText | canCase | canSize | canBold | canAlign},

	{Key: "referral_title", Group: "referral", Label: "Title", Default: defaultReferralTitle, Can: canText | canCase | canSize | canBold | canAlign},
	{Key: "referral_note", Group: "referral", Label: "Line", Default: defaultReferralNote, Can: canText | canCase | canSize | canBold | canAlign},

	{Key: "socials_title", Group: "socials", Label: "Title", Default: defaultSocialsTitle, Can: canText | canCase | canSize | canBold},
	{Key: "socials_handle", Group: "socials", Label: "Handle under the icons", Can: canSize | canBold | canHide},

	{Key: "thanks", Group: "thanks", Label: "Thank-you line", Default: "Thank you for shopping with us!", Can: canAll},
}

// Date formats the receipt can print, by name; "" is the first.
var receiptDateFormats = []struct {
	Key, Label, Layout string
}{
	{"", "19-10-2026 14:30", "02-01-2006 15:04"},
	{"slash", "19/10/2026 14:30", "02/01/2006 15:04"},
	{"slash12", "19/10/2026 2:30 PM", "02/01/2006 3:04 PM"},
	{"iso", "2026-10-19 14:30", "2006-01-02 15:04"},
	{"long", "19 Oct 2026, 14:30", "2 Jan 2006, 15:04"},
	{"longday", "Mon 19 Oct 2026, 14:30", "Mon 2 Jan 2006, 15:04"},
	{"date", "19/10/2026", "02/01/2006"},
}

const (
	receiptElementMinSize = -4
	receiptElementMaxSize = 10
)

func receiptElementByKey(key string) (receiptElementInfo, bool) {
	for _, e := range receiptElements {
		if e.Key == key {
			return e, true
		}
	}
	return receiptElementInfo{}, false
}

func receiptGroupOf(key string) string {
	if e, ok := receiptElementByKey(key); ok {
		return e.Group
	}
	return ""
}

// The older per-part wording (receiptTexts' own fields) still counts, under
// the element it now is.
func (t receiptTexts) element(key string) receiptElement {
	e := t.Elements[key]
	// The part's own settings fill what the line leaves unset.
	if g, ok := t.Groups[receiptGroupOf(key)]; ok {
		info, _ := receiptElementByKey(key)
		if e.Case == "" && info.Can&canCase != 0 {
			e.Case = g.Case
		}
		if e.Size == 0 && info.Can&canSize != 0 {
			e.Size = g.Size
		}
		if e.Bold == nil && info.Can&canBold != 0 {
			e.Bold = g.Bold
		}
		if e.Align == "" && info.Can&canAlign != 0 {
			e.Align = g.Align
		}
	}
	if e.Text == "" {
		switch key {
		case "payments_title":
			e.Text = t.PaymentsTitle
		case "policy_title":
			e.Text = t.PolicyTitle
		case "referral_title":
			e.Text = t.ReferralTitle
		case "referral_note":
			e.Text = t.ReferralNote
		case "socials_title":
			e.Text = t.SocialsTitle
		case "thanks":
			e.Text = t.Thanks
		}
	}
	return e
}

// applyReceiptCase writes s in the element's capitals.
func applyReceiptCase(s, c string) string {
	switch c {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "title":
		var b strings.Builder
		start := true
		for _, r := range s {
			if unicode.IsSpace(r) || r == '-' || r == '/' {
				start = true
				b.WriteRune(r)
				continue
			}
			if start {
				b.WriteRune(unicode.ToUpper(r))
			} else {
				b.WriteRune(unicode.ToLower(r))
			}
			start = false
		}
		return b.String()
	}
	return s
}

// word is what an element prints: its own words or def, in its capitals.
func (t receiptTexts) word(key, def string) string {
	e := t.element(key)
	text := def
	if e.Text != "" {
		text = e.Text
	}
	return applyReceiptCase(text, e.Case)
}

func (t receiptTexts) elementHidden(key string) bool {
	return t.element(key).Hide
}

// dateLayout is the Go layout of the chosen date format.
func (t receiptTexts) dateLayout() string {
	for _, f := range receiptDateFormats {
		if f.Key == t.DateFormat {
			return f.Layout
		}
	}
	return receiptDateFormats[0].Layout
}

// normalizeReceiptElements keeps the elements the till knows, each change
// within what that element allows; an element with nothing left is dropped.
func normalizeReceiptElements(in map[string]receiptElement, clean func(string) string) map[string]receiptElement {
	out := map[string]receiptElement{}
	for key, e := range in {
		info, ok := receiptElementByKey(key)
		if !ok {
			continue
		}
		var n receiptElement
		if info.Can&canText != 0 {
			n.Text = clean(e.Text)
		}
		if info.Can&canCase != 0 && (e.Case == "upper" || e.Case == "lower" || e.Case == "title") {
			n.Case = e.Case
		}
		if info.Can&canSize != 0 {
			size := e.Size
			if size < receiptElementMinSize {
				size = receiptElementMinSize
			}
			if size > receiptElementMaxSize {
				size = receiptElementMaxSize
			}
			n.Size = float64(int(size*2)) / 2 // half points
		}
		if info.Can&canBold != 0 && e.Bold != nil {
			b := *e.Bold
			n.Bold = &b
		}
		if info.Can&canAlign != 0 && (e.Align == "left" || e.Align == "center" || e.Align == "right") {
			n.Align = e.Align
		}
		if info.Can&canHide != 0 {
			n.Hide = e.Hide
		}
		if n != (receiptElement{}) {
			out[key] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validReceiptDateFormat(key string) string {
	for _, f := range receiptDateFormats {
		if f.Key == key {
			return key
		}
	}
	return ""
}

// The canvas side: an element's line drawn as the branch set it.

// elementStyle is an element's size and weight from the look's own.
func (c *receiptCanvas) elementStyle(key string, size float64, bold bool) (float64, bool) {
	e := c.data.Texts.element(key)
	size += e.Size
	if size < 4 {
		size = 4
	}
	if e.Bold != nil {
		bold = *e.Bold
	}
	return size, bold
}

// line draws an element's one line at y: hidden is no line at all (y as it
// was, ok false), else in its size, weight and alignment (align is the look's
// own: "left", "center" or "right"). It marks the line for the designer.
func (c *receiptCanvas) line(y int, key, value string, size float64, bold, title bool, align string) (int, bool, error) {
	e := c.data.Texts.element(key)
	if e.Hide || strings.TrimSpace(value) == "" {
		return y, false, nil
	}
	size, bold = c.elementStyle(key, size, bold)
	if e.Align != "" {
		align = e.Align
	}
	top := y
	var err error
	switch align {
	case "left":
		y, err = c.text(c.margin, y, value, size, bold, title)
	case "right":
		w, mErr := c.measure(value, size, bold, title)
		if mErr != nil {
			return y, false, mErr
		}
		y, err = c.text(c.margin+c.contentW-w, y, value, size, bold, title)
	default:
		y, err = c.center(y, value, size, bold, title)
	}
	c.markElement(key, top, y)
	c.markLine(key, value, top, y, size, align)
	return y, true, err
}

// lineWrapped is line for words that may need more than one line.
func (c *receiptCanvas) lineWrapped(y int, key, value string, size float64, bold bool, spacing int, align string) (int, bool, error) {
	e := c.data.Texts.element(key)
	if e.Hide || strings.TrimSpace(value) == "" {
		return y, false, nil
	}
	size, bold = c.elementStyle(key, size, bold)
	if e.Align != "" {
		align = e.Align
	}
	top := y
	var err error
	y, err = c.wrappedAligned(y, value, size, bold, spacing, align)
	c.markElement(key, top, y)
	c.markLine(key, value, top, y, size, align)
	return y, true, err
}

// wrappedAligned wraps value to the paper and draws each line aligned.
func (c *receiptCanvas) wrappedAligned(y int, value string, size float64, bold bool, spacing int, align string) (int, error) {
	role := roleBody
	if bold {
		role = roleBodyBold
	}
	face, err := c.faceRole(size, role)
	if err != nil {
		return y, err
	}
	var lines []string
	for _, para := range strings.Split(value, "\n") {
		words := strings.Fields(para)
		current := ""
		for _, w := range words {
			next := strings.TrimSpace(current + " " + w)
			if current != "" && measureFace(face, next) > c.contentW {
				lines = append(lines, current)
				current = w
				continue
			}
			current = next
		}
		lines = append(lines, current)
	}
	for _, l := range lines {
		width := measureFace(face, l)
		x := c.margin + (c.contentW-width)/2
		switch align {
		case "left":
			x = c.margin
		case "right":
			x = c.margin + c.contentW - width
		}
		if y, err = c.textRole(x, y, l, size, role, blackInk); err != nil {
			return y, err
		}
		y += spacing
	}
	return y, nil
}

// markElement records where an element's line is, under its group.
func (c *receiptCanvas) markElement(key string, top, bottom int) {
	if group := receiptGroupOf(key); group != "" && bottom > top {
		c.marks = append(c.marks, receiptMark{Kind: "group", Part: group, Top: top, Bottom: bottom})
	}
}

// markGroup records a part of the receipt the designer opens as one.
func (c *receiptCanvas) markGroup(group string, top, bottom int) {
	if bottom > top {
		c.marks = append(c.marks, receiptMark{Kind: "group", Part: group, Top: top, Bottom: bottom})
	}
}

// receiptTextLen counts what the till keeps of a text.
func receiptTextLen(s string) int { return utf8.RuneCountInString(s) }

// invoiceLabel is the invoice's title by its kind, in the branch's words.
func (c *receiptCanvas) invoiceLabel() string {
	label := receiptInvoiceLabel(c.data.ReceiptNo)
	switch label {
	case "Exchange Invoice":
		return c.data.Texts.word("invoice_label_exchange", label)
	case "Return Invoice":
		return c.data.Texts.word("invoice_label_return", label)
	}
	return c.data.Texts.word("invoice_label_sale", label)
}

// gridMidHeader is the middle column's title in the branch's words.
func (c *receiptCanvas) gridMidHeader(cfg gridConfig) string {
	if cfg.midIsPrice {
		return c.data.Texts.word("col_price", "PRICE")
	}
	return c.data.Texts.word("col_disc", cfg.midHeader)
}

// extraHeight is room, in pixels, a design may add to the receipt: its own
// words (a long one wraps) and lines made bigger. An over-estimate, since
// the canvas is cut to what was drawn.
func (t receiptTexts) extraHeight() int {
	extra := 0
	for _, e := range t.Elements {
		grow := 1.0
		if e.Size > 0 {
			grow += e.Size / 4
		}
		extra += int(float64(80+6*receiptTextLen(e.Text)) * grow)
	}
	return extra
}

// now is the receipt's date: the one it was given, else the clock's.
func (c *receiptCanvas) now() time.Time {
	if !c.data.Now.IsZero() {
		return c.data.Now
	}
	return receiptNow()
}

// normalizeReceiptGroups keeps a part's capitals, size, weight and alignment.
func normalizeReceiptGroups(in map[string]receiptElement) map[string]receiptElement {
	out := map[string]receiptElement{}
	for key, g := range in {
		known := false
		for _, info := range receiptGroups {
			known = known || info.Key == key
		}
		if !known {
			continue
		}
		var n receiptElement
		if g.Case == "upper" || g.Case == "lower" || g.Case == "title" {
			n.Case = g.Case
		}
		n.Size = float64(int(min(max(g.Size, receiptElementMinSize), receiptElementMaxSize)*2)) / 2
		if g.Bold != nil {
			b := *g.Bold
			n.Bold = &b
		}
		if g.Align == "left" || g.Align == "center" || g.Align == "right" {
			n.Align = g.Align
		}
		if n != (receiptElement{}) {
			out[key] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// The parts under the totals, in their default order; Order rearranges them.
var receiptFooterOrder = []string{receiptSectionPayments, receiptSectionPolicy, receiptSectionReferral, receiptSectionSocials, receiptSectionThanks}

// normalizeReceiptOrder keeps the known footer parts, once each, and nil
// when that is the default order.
func normalizeReceiptOrder(in []string) []string {
	var out []string
	for _, key := range in {
		if slices.Contains(receiptFooterOrder, key) && !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	for _, key := range receiptFooterOrder {
		if !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	if slices.Equal(out, receiptFooterOrder) {
		return nil
	}
	return out
}

// footerOrder is the order the footer parts print in.
func (t receiptTexts) footerOrder() []string {
	if order := normalizeReceiptOrder(t.Order); order != nil {
		return order
	}
	return receiptFooterOrder
}

// row is a label and an amount on one line in an element's size and
// weight; word is the element's own words within the label, for the designer
// to edit in place.
func (c *receiptCanvas) row(y int, key, word, label, amount string, size float64, bold bool) int {
	size, bold = c.elementStyle(key, size, bold)
	top := y
	y, _ = c.twoCol(y, label, amount, size, bold)
	c.markLine(key, word, top, y, size, "left")
	return y
}

// markLine records where an element's line is and what it says, so the
// designer can type over it on the paper.
func (c *receiptCanvas) markLine(key, text string, top, bottom int, size float64, align string) {
	if bottom <= top {
		return
	}
	c.marks = append(c.marks, receiptMark{
		Kind: "line", Part: key, Top: top, Bottom: bottom, Text: text,
		SizePx: size * c.bodyFactor() * 2.6, Left: alignCode(align),
	})
}

// alignCode keeps a line's alignment in a mark's Left: 1 left, 2 centre,
// 3 right.
func alignCode(align string) int {
	switch align {
	case "left":
		return 1
	case "right":
		return 3
	}
	return 2
}
