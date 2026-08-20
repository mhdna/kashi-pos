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
	if data.Description != "" {
		for _, line := range strings.Split(data.Description, "\n") {
			writeESCPosCentered(&out, line, columns, false)
		}
		writeESCPosCentered(&out, "Invoice# "+data.ReceiptNo, columns, false)
		writeESCPosCentered(&out, currentReceiptDate(), columns, false)
	}
	branchPhone := data.BranchPhone
	if branchPhone == "" {
		branchPhone = data.Phone
	}
	if branchPhone != "" {
		writeESCPosCentered(&out, branchPhone, columns, false)
	}
	if data.Website != "" {
		writeESCPosCentered(&out, data.Website, columns, false)
	}
	out.WriteString(separator + "\n")
	writeESCPosLine(&out, "Sold by:", fallbackReceiptValue(data.Salesperson, "Not specified"), columns)
	writeESCPosLine(&out, "Customer:", fallbackReceiptValue(data.Customer, "Walk-in"), columns)
	writeESCPosLine(&out, "Register:", fallbackReceiptValue(data.RegisterName, "Not specified"), columns)
	if strings.TrimSpace(data.Notes) != "" {
		writeESCPosText(&out, "Note: "+strings.TrimSpace(data.Notes))
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
	out.WriteString(separator + "\n")
	writeESCPosLine(&out, "Subtotal", fmt.Sprintf("$%.2f", subtotal), columns)
	writeESCPosLine(&out, "Discount", fmt.Sprintf("-$%.2f", totalDiscount), columns)
	writeESCPosLine(&out, "Tax", fmt.Sprintf("$%.2f", tax), columns)
	out.WriteString(separator + "\n")
	writeESCPosBoldLine(&out, "TOTAL", fmt.Sprintf("$%.2f", grandTotal), columns)
	if currency := data.RoundingCurrency; currency.UnitsPerUsdMicros > 0 {
		if roundedUnits, err := roundCurrencyUnitsFromCents(data.GrandTotalCents, currency.UnitsPerUsdMicros, currency.CashRoundingUnit, data.RoundingMode); err == nil {
			writeESCPosLine(&out, currency.Code+" TOTAL", fmt.Sprintf("%s %s", formatInt(roundedUnits), currency.Symbol), columns)
		}
	}
	writeESCPosCentered(&out, "PAYMENT DETAILS", columns, true)
	for _, payment := range data.Payments {
		label := payment.AccountName
		if label == "" {
			label = "Cash"
		}
		if payment.EntryType == "change" {
			label = "CHANGE RETURNED " + label
		} else {
			label = "CASH RECEIVED " + label
		}
		writeESCPosLine(&out, label, formatReceiptPayment(payment, data.RoundingMode), columns)
	}
	out.WriteString(separator + "\n")
	writeESCPosCentered(&out, "EXCHANGE POLICY", columns, true)
	policy := data.ReturnPolicy
	if policy == "" {
		policy = "Exchanges accepted within 3 days."
	}
	writeESCPosText(&out, "")
	for _, line := range strings.Split(policy, "\n") {
		writeESCPosText(&out, line)
	}
	writeESCPosText(&out, "")
	if len(data.SocialPlatforms) > 0 {
		writeESCPosCentered(&out, "Follow us:", columns, true)
		socialLines := formatSocialFooterLines(data.SocialPlatforms, data.SocialHandles)
		for _, line := range socialLines {
			writeESCPosCentered(&out, line, columns, false)
		}
	}
	out.WriteString(separator + "\n")
	writeESCPosCentered(&out, "Thank you for shopping with us!", columns, false)
	writeESCPosCentered(&out, data.ReceiptNo, columns, false)
	out.WriteString("\n\n\n")
	// Keep the historical default for callers that do not provide receipt settings.
	if data.Cutoff || data.Font == "" {
		// GS V 0 selects a full cut. The previous parameter (3) is not a
		// standard ESC/POS cut mode and is ignored by many printers.
		out.Write([]byte{gs, cutMode, 0})
	}
	return out.Bytes()
}

func fallbackReceiptValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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

func formatSocialFooterLines(platforms []string, handles map[string]string) []string {
	lines := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		handle := strings.TrimSpace(handles[platform])
		if handle == "" {
			continue
		}
		lines = append(lines, socialPlatformMarker(platform)+" "+handle)
	}
	return lines
}

func socialPlatformMarker(platform string) string {
	switch platform {
	case "facebook":
		return "[f]"
	case "instagram":
		return "[ig]"
	case "threads":
		return "[@]"
	case "youtube":
		return "[yt]"
	case "tiktok":
		return "[♪]"
	default:
		return "[@]"
	}
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
	return time.Now().Format("02-01-2006 15:04")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
