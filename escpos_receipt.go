package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	esc     = 0x1b
	gs      = 0x1d
	cutMode = 0x56
)

func buildESCPosReceipt(data printReceiptData) []byte {
	width := int(data.ReceiptWidth)
	if width <= 0 {
		width = 80
	}
	columns := 48
	if width <= 58 {
		columns = 32
	}
	separator := strings.Repeat("-", columns)

	var out bytes.Buffer
	out.Write([]byte{esc, '@'})
	writeESCPosCentered(&out, data.StoreName, columns, true)
	writeESCPosCentered(&out, data.Branch, columns, false)
	if data.Phone != "" {
		writeESCPosCentered(&out, data.Phone, columns, false)
	}
	out.WriteString(separator + "\n")
	writeESCPosLine(&out, "Invoice:", data.ReceiptNo, columns)
	writeESCPosLine(&out, "Date:", currentReceiptDate(), columns)
	if data.Salesperson != "" {
		writeESCPosLine(&out, "Salesperson:", data.Salesperson, columns)
	}
	out.WriteString(separator + "\n")
	writeESCPosLine(&out, "ITEM", "TOTAL", columns)
	out.WriteString(separator + "\n")

	var subtotal, totalDiscount float64
	for _, item := range data.Items {
		gross := item.Qty * item.Price
		discount := gross * item.Discount / 100
		lineTotal := gross - discount
		subtotal += gross
		totalDiscount += discount
		writeESCPosLine(&out, sanitizeESCPos(item.Name), fmt.Sprintf("$%.2f", lineTotal), columns)
		qtyLine := fmt.Sprintf("%.0f x $%.2f", item.Qty, item.Price)
		if item.Discount > 0 {
			qtyLine += fmt.Sprintf(" (-%.0f%%)", item.Discount)
		}
		writeESCPosText(&out, "  "+qtyLine)
	}

	grandTotal := float64(data.GrandTotalCents) / 100
	tax := grandTotal - (subtotal - totalDiscount)
	rate := data.ExchangeRate
	if rate <= 0 {
		rate = 90000
	}
	out.WriteString(separator + "\n")
	writeESCPosLine(&out, "Subtotal", fmt.Sprintf("$%.2f", subtotal), columns)
	writeESCPosLine(&out, "Discount", fmt.Sprintf("-$%.2f", totalDiscount), columns)
	writeESCPosLine(&out, "Tax", fmt.Sprintf("$%.2f", tax), columns)
	out.WriteString(separator + "\n")
	writeESCPosBoldLine(&out, "TOTAL", fmt.Sprintf("$%.2f", grandTotal), columns)
	writeESCPosLine(&out, "LBP TOTAL", fmt.Sprintf("%.0f L.L.", grandTotal*float64(rate)), columns)
	out.WriteString(separator + "\n")
	writeESCPosCentered(&out, "Thank you for shopping with us!", columns, false)
	writeESCPosCentered(&out, data.ReceiptNo, columns, false)
	out.WriteString("\n\n\n")
	out.Write([]byte{gs, cutMode, 3})
	return out.Bytes()
}

func writeESCPosCentered(out *bytes.Buffer, value string, columns int, bold bool) {
	value = sanitizeESCPos(value)
	if len([]rune(value)) > columns {
		value = string([]rune(value)[:columns])
	}
	padding := (columns - utf8.RuneCountInString(value)) / 2
	if bold {
		out.Write([]byte{esc, 'E', 1})
	}
	writeESCPosText(out, strings.Repeat(" ", padding)+value)
	if bold {
		out.Write([]byte{esc, 'E', 0})
	}
}

func writeESCPosLine(out *bytes.Buffer, left, right string, columns int) {
	writeESCPosLineMode(out, left, right, columns, false)
}

func writeESCPosBoldLine(out *bytes.Buffer, left, right string, columns int) {
	writeESCPosLineMode(out, left, right, columns, true)
}

func writeESCPosLineMode(out *bytes.Buffer, left, right string, columns int, bold bool) {
	left, right = sanitizeESCPos(left), sanitizeESCPos(right)
	leftWidth := columns - utf8.RuneCountInString(right) - 1
	if leftWidth < 1 {
		leftWidth = 1
	}
	leftRunes := []rune(left)
	if len(leftRunes) > leftWidth {
		left = string(leftRunes[:maxInt(1, leftWidth-1)]) + "~"
	}
	spaces := columns - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
	if spaces < 1 {
		spaces = 1
	}
	if bold {
		out.Write([]byte{esc, 'E', 1})
	}
	writeESCPosText(out, left+strings.Repeat(" ", spaces)+right)
	if bold {
		out.Write([]byte{esc, 'E', 0})
	}
}

func writeESCPosText(out *bytes.Buffer, value string) {
	out.WriteString(sanitizeESCPos(value))
	out.WriteByte('\n')
}

func sanitizeESCPos(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			out.WriteRune(' ')
		case r >= 0x20 && r <= 0x7e:
			out.WriteRune(r)
		case r == utf8.RuneError || r > 0x7e:
			out.WriteRune('?')
		default:
			out.WriteRune('?')
		}
	}
	return out.String()
}

func currentReceiptDate() string {
	return time.Now().Format("02/01/2006 15:04")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
