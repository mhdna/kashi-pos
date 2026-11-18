package main

// Receipt part marks: where each part central's Receipt Designer can reword
// or leave out lands on the drawn receipt, so the designer can lay its own
// editable text over the till's real picture (receipt_preview_test.go writes
// them into the preview catalog).

// receiptMark is one part's band of the image, top to bottom in pixels of
// the cropped receipt. Text parts also say how their text is drawn (size in
// pixels, bold), so an edited one can be drawn alike; a section's band runs
// from where it starts to where the next part starts, so leaving it out
// closes the gap as the till does.
type receiptMark struct {
	// Kind is "section" (one of receiptSections, which can be left out),
	// "text" (one receiptTexts wording) or "icons" (the socials row's icons).
	Kind   string `json:"kind"`
	Part   string `json:"part"`
	Top    int    `json:"top"`
	Bottom int    `json:"bottom"`
	// Left and Right, when set, are the part's own x range (the socials
	// row's title and its icons share one line).
	Left   int     `json:"left,omitempty"`
	Right  int     `json:"right,omitempty"`
	Text   string  `json:"text,omitempty"`
	SizePx float64 `json:"size_px,omitempty"`
	Bold   bool    `json:"bold,omitempty"`
	// X0 and X1, when set, are a line's own x range: words that share a row
	// with others (column titles, a row's label).
	X0 int `json:"x0,omitempty"`
	X1 int `json:"x1,omitempty"`
}

// Text parts: one per receiptTexts wording.
const (
	receiptPartPaymentsTitle = "payments_title"
	receiptPartPolicyTitle   = "policy_title"
	receiptPartReferralTitle = "referral_title"
	receiptPartReferralNote  = "referral_note"
	receiptPartSocialsTitle  = "socials_title"
	receiptPartThanks        = "thanks"
	// The socials row's icons, kept beside an edited title.
	receiptPartSocialsIcons = "socials_icons"
)

// markSection records a section (one of receiptSections) from top to bottom.
func (c *receiptCanvas) markSection(section string, top, bottom int) {
	c.marks = append(c.marks, receiptMark{Kind: "section", Part: section, Top: top, Bottom: bottom})
}

// markText records a text part drawn from top to bottom at size, in the
// pixels the face is built at (see faceRole).
func (c *receiptCanvas) markText(part, text string, top, bottom int, size float64, bold bool) {
	c.marks = append(c.marks, receiptMark{
		Kind: "text", Part: part, Top: top, Bottom: bottom, Text: text,
		SizePx: size * c.bodyFactor() * 2.6, Bold: bold,
	})
}

// shiftMarks moves every mark up by top, the rows cropped off the image.
func shiftMarks(marks []receiptMark, top int) []receiptMark {
	out := make([]receiptMark, 0, len(marks))
	for _, m := range marks {
		m.Top -= top
		m.Bottom -= top
		if m.Top < 0 {
			m.Top = 0
		}
		// A part that drew nothing (every line of it hidden) has no place.
		if m.Bottom <= m.Top {
			continue
		}
		out = append(out, m)
	}
	return out
}
