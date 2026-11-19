package main

import (
	"image"
	"regexp"
	"slices"
	"strings"
	"time"
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
	// Show prints a part that is left out unless asked for (canShow).
	Show bool `json:"show,omitempty"`
	// Font is one of receiptFonts; "" the look's own.
	Font string `json:"font,omitempty"`
}

// What an element can change.
const (
	canText  = 1 << iota // its words
	canCase              // capitals
	canSize              // size
	canBold              // weight
	canAlign             // alignment
	canHide              // leave it out
	canFont              // its font
	canShow              // print it: for a part that is off unless switched on
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
	{Key: "meta", Label: "Customer, salesman and date"},
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
	{Key: "meta_salesperson", Group: "meta", Label: "Salesman", Default: "Salesman", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "meta_date", Group: "meta", Label: "Date", Default: "Date", Can: canText | canCase | canSize | canBold | canHide},
	{Key: "note_label", Group: "meta", Label: "Note", Default: "Note", Can: canText | canCase},

	// What each item row prints besides its amounts (itemsAsPrinted).
	{Key: "item_name", Group: "columns", Label: "Name", Can: canHide},
	{Key: "item_variant", Group: "columns", Label: "Colour and size", Can: canHide},
	{Key: "item_code", Group: "columns", Label: "Code", Can: canShow},
	{Key: "item_price_note", Group: "columns", Label: "Before and after price", Can: canHide},
	{Key: "deal_free", Group: "columns", Label: "Free item tag", Default: "(FREE)", Can: canText | canCase | canHide},
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
	{Key: "invoice_discount", Group: "totals", Label: "Deal on the invoice", Default: "{name}", Can: canText | canCase | canSize | canBold},
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
		if e.Font == "" && info.Can&canFont != 0 {
			e.Font = g.Font
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

// receiptPlaceholder is a word the till fills in ({name}, {percent},
// {currency}); capitals never touch it, so it is still found and filled.
var receiptPlaceholder = regexp.MustCompile(`\{[a-z]+\}`)

// applyReceiptCase writes s in the element's capitals, its placeholders as
// they are.
func applyReceiptCase(s, c string) string {
	if c == "" {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range receiptPlaceholder.FindAllStringIndex(s, -1) {
		b.WriteString(applyCaseRun(s[last:loc[0]], c))
		b.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(applyCaseRun(s[last:], c))
	return b.String()
}

func applyCaseRun(s, c string) string {
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
		if info.Can&canFont != 0 {
			n.Font = validReceiptFont(e.Font)
		}
		if info.Can&canHide != 0 {
			n.Hide = e.Hide
		}
		if info.Can&canShow != 0 {
			n.Show = e.Show
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
	defer c.useFont(e.Font)()
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
	defer c.useFont(e.Font)()
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
		return c.word("col_price", "PRICE")
	}
	return c.word("col_disc", cfg.midHeader)
}

// extraHeight is room, in pixels, a design may add to the receipt: its own
// words (a long one wraps) and lines made bigger. An over-estimate, since
// the canvas is cut to what was drawn.
func (t receiptTexts) extraHeight() int {
	extra := 320 * len(t.Banners)
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
		n.Font = validReceiptFont(g.Font)
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
	defer c.useFont(c.data.Texts.element(key).Font)()
	size, bold = c.elementStyle(key, size, bold)
	top := y
	y, _ = c.twoCol(y, label, amount, size, bold)
	c.markLine(key, word, top, y, size, "left")
	if w, err := c.measure(word, size, bold, false); err == nil {
		m := &c.marks[len(c.marks)-1]
		m.X0, m.X1 = c.margin, c.margin+w
	}
	return y
}

// dealRows is the invoice deal's row: as row when its words fit beside the
// amount, else a line per few deals (splitDealWords), the amount on the last.
func (c *receiptCanvas) dealRows(y int, label, amount string) int {
	defer c.useFont(c.data.Texts.element("invoice_discount").Font)()
	size, bold := c.elementStyle("invoice_discount", 10, false)
	amountW, err := c.measure(amount, size, bold, false)
	if err != nil {
		return c.row(y, "invoice_discount", label, label, amount, 10, false)
	}
	lines := splitDealWords(label, func(s string) bool {
		w, err := c.measure(s, size, bold, false)
		return err == nil && w+amountW+16 <= c.contentW
	})
	for _, l := range lines[:len(lines)-1] {
		y, _ = c.text(c.margin, y, l, size, bold, false)
		y += c.gap(2)
	}
	last := lines[len(lines)-1]
	return c.row(y, "invoice_discount", last, last, amount, 10, false)
}

// markLine records where an element's line is and what it says, so the
// designer can type over it on the paper.
func (c *receiptCanvas) markLine(key, text string, top, bottom int, size float64, align string) {
	if bottom <= top {
		return
	}
	scale := c.bodyFactor()
	if f, ok := receiptFontByKey(c.fontKey); ok {
		scale = f.Scale
	}
	c.marks = append(c.marks, receiptMark{
		Kind: "line", Part: key, Top: top, Bottom: bottom, Text: text,
		SizePx: size * scale * 2.6, Left: alignCode(align),
		Font: c.lastFont, Bold: c.lastBold,
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

// pendingWord is words about to be drawn (text) for an element whose own
// words (word) are part of them.
type pendingWord struct{ key, text, word string }

// word is an element's words for a line drawn among others on a row (a
// column title): the next time they are drawn, where they land is marked.
func (c *receiptCanvas) word(key, def string) string {
	w := c.data.Texts.word(key, def)
	c.pendingWords = append(c.pendingWords, pendingWord{key, w, w})
	return w
}

// wordWith is word followed by fixed words drawn with it ("ITEM / CODE").
func (c *receiptCanvas) wordWith(key, def, after string) string {
	w := c.data.Texts.word(key, def)
	c.pendingWords = append(c.pendingWords, pendingWord{key, w + after, w})
	return w + after
}

// pendingFont is the font set for words about to be drawn among others on
// a row (a column title, TOTAL), when they are pending; "" for none.
func (c *receiptCanvas) pendingFont(value string) string {
	for _, p := range c.pendingWords {
		if p.text == value {
			return c.data.Texts.element(p.key).Font
		}
	}
	return ""
}

// markPendingWord marks words just drawn at x when they are a pending word.
func (c *receiptCanvas) markPendingWord(value string, x, top, bottom int, size float64, role fontRole, width int) {
	for i, p := range c.pendingWords {
		if p.text != value {
			continue
		}
		scale := c.bodyFactor()
		if role == roleTitle || role == roleTitleBold {
			scale = c.titleFactor()
		}
		if f, ok := receiptFontByKey(c.fontKey); ok {
			scale = f.Scale
		}
		c.marks = append(c.marks, receiptMark{
			Kind: "line", Part: p.key, Top: top, Bottom: bottom, Text: p.word,
			SizePx: size * scale * 2.6, Left: alignCode("left"), X0: x, X1: x + width,
			Font: c.lastFont, Bold: c.lastBold,
		})
		c.pendingWords = append(c.pendingWords[:i], c.pendingWords[i+1:]...)
		// The same words measured first are pending again; one mark each.
		for j := len(c.pendingWords) - 1; j >= 0; j-- {
			if c.pendingWords[j].key == p.key {
				c.pendingWords = append(c.pendingWords[:j], c.pendingWords[j+1:]...)
			}
		}
		return
	}
}

// splitDealWords breaks the deal line's words into lines when they don't fit
// (fits says whether a line fits beside the amount): deals that added up are
// joined with " + ", and each line holds as many as fit, ending on " +". One
// line, as it was, when they fit or there is nothing to break.
func splitDealWords(words string, fits func(string) bool) []string {
	if fits(words) || !strings.Contains(words, " + ") {
		return []string{words}
	}
	parts := strings.Split(words, " + ")
	var lines []string
	cur := parts[0]
	for _, p := range parts[1:] {
		if next := cur + " + " + p; fits(next) {
			cur = next
			continue
		}
		lines = append(lines, cur+" +")
		cur = p
	}
	return append(lines, cur)
}

// invoiceDiscountWords is the deal's line label in the branch's words;
// {name} is the deal's own name ("Students 15%").
func (t receiptTexts) invoiceDiscountWords(d invoiceDiscount) string {
	e := t.element("invoice_discount")
	text := strings.ReplaceAll(orDefault(e.Text, "{name}"), "{name}", invoiceDiscountLabel(d))
	return applyReceiptCase(text, e.Case)
}

// freeTag is the words after a unit a deal gave free ("Shirt (FREE)"): the
// till names it so, and the branch can reword the tag or leave it off.
const freeTag = " (FREE)"

// itemsAsPrinted is the items as their rows print, after the designer's
// item settings: the free unit's tag in the shop's words (or none), and which
// of name, colour and size, code and the before-and-after line each row
// shows. A row whose name is left out shows its code in its place when the
// code is on, so it still says what was bought.
func (c *receiptCanvas) itemsAsPrinted(items []invoiceItem) []invoiceItem {
	return itemsAsPrinted(c.data.Texts, items)
}

func itemsAsPrinted(t receiptTexts, items []invoiceItem) []invoiceItem {
	free := t.element("deal_free")
	hideName := t.elementHidden("item_name")
	hideVariant := t.elementHidden("item_variant")
	showCode := t.element("item_code").Show
	hideNote := t.elementHidden("item_price_note")
	out := make([]invoiceItem, len(items))
	for i, item := range items {
		tagged := item.Free && strings.HasSuffix(item.Name, freeTag)
		name := strings.TrimSuffix(item.Name, freeTag)
		if !tagged {
			name = item.Name
		}
		code := ""
		if showCode {
			code = itemCodeText(item)
		}
		if hideName {
			name, code = code, ""
		}
		if tagged && !free.Hide {
			tag := freeTag
			if free.Text != "" || free.Case != "" {
				tag = " " + t.word("deal_free", "(FREE)")
			}
			name = strings.TrimSpace(name + tag)
		}
		item.Name = name
		item.PrintCode = code
		if hideVariant {
			item.Color, item.Size = "", ""
		}
		item.NoPriceNote = hideNote
		out[i] = item
	}
	return out
}
