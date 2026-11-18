package main

import (
	"image/color"
	"math"
)

// A part's lines and box (receiptTexts.Decor), set from central's Receipt
// Designer: a line above it, the line under it (in place of the look's own
// divider, or none), and a box around it, each dotted, dashed, solid or
// double, in a thickness, the box's corners square or rounded. Nothing set
// prints the look as it was.

type receiptDecor struct {
	// Above and Below are a line style ("dotted", "dashed", "solid",
	// "double"); Below "none" takes away the look's own divider under the
	// part, "" keeps it.
	Above string `json:"above,omitempty"`
	Below string `json:"below,omitempty"`
	// Box is the style of a box around the part, "" for none.
	Box     string `json:"box,omitempty"`
	Rounded bool   `json:"rounded,omitempty"`
	// Thickness is the lines' weight, 1 (thin) to 4; 0 is 2.
	Thickness int `json:"thickness,omitempty"`
}

var receiptLineStyles = []string{"dotted", "dashed", "solid", "double"}

func validLineStyle(s string, allowNone bool) string {
	for _, v := range receiptLineStyles {
		if s == v {
			return s
		}
	}
	if allowNone && s == "none" {
		return s
	}
	return ""
}

func normalizeReceiptDecor(in map[string]receiptDecor) map[string]receiptDecor {
	out := map[string]receiptDecor{}
	for key, d := range in {
		known := false
		for _, g := range receiptGroups {
			known = known || g.Key == key
		}
		if !known {
			continue
		}
		n := receiptDecor{
			Above: validLineStyle(d.Above, false), Below: validLineStyle(d.Below, true),
			Box: validLineStyle(d.Box, false), Rounded: d.Rounded,
		}
		if n.Box == "" {
			n.Rounded = false
		}
		if n.Above != "" || (n.Below != "" && n.Below != "none") || n.Box != "" {
			n.Thickness = min(max(d.Thickness, 0), 4)
		}
		if n != (receiptDecor{}) {
			out[key] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (c *receiptCanvas) decor(group string) receiptDecor {
	return c.data.Texts.Decor[group]
}

func decorThickness(d receiptDecor) int {
	if d.Thickness <= 0 {
		return 2
	}
	return d.Thickness
}

// divider is the line under a part that has one in the look: the look's own,
// none, or the style chosen. It returns the y below it.
func (c *receiptCanvas) divider(group string, y int) int {
	d := c.decor(group)
	switch d.Below {
	case "":
		return c.ruleLine(y)
	case "none":
		return y
	}
	return c.styledRule(y, d.Below, decorThickness(d))
}

// decorStart draws a part's line above and opens its box, returning where
// the part's own lines start and where the box begins.
func (c *receiptCanvas) decorStart(group string, y int) (int, int) {
	d := c.decor(group)
	if d.Above != "" {
		y = c.styledRule(y, d.Above, decorThickness(d)) + c.gap(10)
	}
	boxTop := y
	if d.Box != "" {
		y += 12 + decorThickness(d)
	}
	return y, boxTop
}

// decorEnd closes a part's box, and draws a line under a part that has no
// divider of its own in the look (builtIn false), returning the y below.
func (c *receiptCanvas) decorEnd(group string, y, boxTop int, builtIn bool) int {
	d := c.decor(group)
	if d.Box != "" {
		y += 12 + decorThickness(d)
		c.styledBox(boxTop, y, d.Box, decorThickness(d), d.Rounded)
		y += c.gap(8)
	}
	if !builtIn && d.Below != "" && d.Below != "none" {
		y = c.styledRule(y, d.Below, decorThickness(d)) + c.gap(10)
	}
	return y
}

// styledRule is a line across the paper in a style and thickness.
func (c *receiptCanvas) styledRule(y int, style string, th int) int {
	left, right := c.margin, c.margin+c.contentW
	return c.hLine(left, right, y, style, th)
}

func (c *receiptCanvas) hLine(left, right, y int, style string, th int) int {
	switch style {
	case "dotted":
		step := th * 3
		for x := left; x < right; x += step {
			c.fillRect(x, y, minInt(x+th, right), y+th, color.Black)
		}
		return y + th
	case "dashed":
		for x := left; x < right; x += 8 + th*4 {
			c.fillRect(x, y, minInt(x+6+th*3, right), y+th, color.Black)
		}
		return y + th
	case "double":
		c.fillRect(left, y, right, y+th, color.Black)
		c.fillRect(left, y+th+3, right, y+2*th+3, color.Black)
		return y + 2*th + 3
	}
	c.fillRect(left, y, right, y+th, color.Black)
	return y + th
}

func (c *receiptCanvas) vLine(x, top, bottom int, style string, th int) {
	switch style {
	case "dotted":
		for y := top; y < bottom; y += th * 3 {
			c.fillRect(x, y, x+th, minInt(y+th, bottom), color.Black)
		}
	case "dashed":
		for y := top; y < bottom; y += 8 + th*4 {
			c.fillRect(x, y, x+th, minInt(y+6+th*3, bottom), color.Black)
		}
	case "double":
		c.fillRect(x, top, x+th, bottom, color.Black)
		c.fillRect(x+th+3, top, x+2*th+3, bottom, color.Black)
	default:
		c.fillRect(x, top, x+th, bottom, color.Black)
	}
}

// styledBox is a box from top to bottom just outside the paper's text.
func (c *receiptCanvas) styledBox(top, bottom int, style string, th int, rounded bool) {
	left := c.margin - 12
	right := c.margin + c.contentW + 12
	r := 0
	if rounded {
		r = 14
	}
	inner := 0
	if style == "double" {
		inner = th + 3
	}
	c.hLine(left+r, right-r, top, style, th)
	c.hLine(left+r, right-r, bottom-th-inner, style, th)
	c.vLine(left, top+r, bottom-r, style, th)
	c.vLine(right-th-inner, top+r, bottom-r, style, th)
	if r > 0 {
		c.arc(left+r, top+r, r, th, math.Pi, 1.5*math.Pi)
		c.arc(right-r, top+r, r, th, 1.5*math.Pi, 2*math.Pi)
		c.arc(left+r, bottom-r, r, th, 0.5*math.Pi, math.Pi)
		c.arc(right-r, bottom-r, r, th, 0, 0.5*math.Pi)
	}
}

// arc is a quarter of a rounded corner, a solid stroke th thick, from angle
// a0 to a1 (radians, y down) around cx, cy.
func (c *receiptCanvas) arc(cx, cy, r, th int, a0, a1 float64) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := float64(x-cx)+0.5, float64(y-cy)+0.5
			d := math.Hypot(dx, dy)
			if d > float64(r) || d < float64(r-th) {
				continue
			}
			a := math.Atan2(dy, dx)
			if a < 0 {
				a += 2 * math.Pi
			}
			if a >= a0 && a <= a1 {
				c.fillRect(x, y, x+1, y+1, color.Black)
			}
		}
	}
}
