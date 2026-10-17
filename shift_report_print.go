package main

import (
	"fmt"
	"strings"
)

type shiftReportData struct {
	ShiftID         int64
	Cashier         string
	Register        string
	OpenedAt        string
	ClosedAt        string
	FirstInvoice    string
	LastInvoice     string
	Transactions    int64
	ItemsSold       int64
	Returns         int64
	GrossSales      int64
	Discounts       int64
	PriceDeductions int64
	ReturnAmount    int64
	Tax             int64
	CashRounding    int64
	NetSales        int64
	TotalPayments   int64
	CountedTotal    int64
	CountedCoupon   int64
	OnAccount       int64
	Payments        []receiptPayment
	Spend           []shiftSpendLine
	// SpendUSD is Spend converted to USD cents; NET TOTAL is TotalPayments
	// (collected, so on-account is already out) minus it.
	SpendUSD        int64
	OpeningFloatUSD int64
	PaidOutUSD      int64
	ExpectedUSD     int64
	CountedUSD      int64
	USDBillCounts   []shiftDenominationCount
	OpeningFloatLBP int64
	PaidOutLBP      int64
	ExpectedLBP     int64
	CountedLBP      int64
	ExpectedVisa    int64
	CountedVisa     int64
	ExpectedWhish   int64
	CountedWhish    int64
	ExchangeRate    int64
	RoundingMode    string
	// SignatureLines prints the cashier/manager sign-off lines at the foot
	// (see shiftReportSignaturesFlag).
	SignatureLines bool
}

type shiftDenominationCount struct {
	Denomination int64
	Count        int64
}

func (c *receiptCanvas) drawShiftReport() (int, error) {
	report := c.data.ShiftReport
	if report == nil {
		return 0, fmt.Errorf("shift report data is required")
	}

	y := 0
	var err error
	if y, err = c.center(y, c.data.StoreName, c.data.TitleSize, true, true); err != nil {
		return 0, err
	}
	y += 4
	branchSize := c.data.TitleSize - 8
	if branchSize <= 0 {
		branchSize = c.data.BodySize
	}
	if strings.TrimSpace(c.data.Branch) != "" {
		y, _ = c.center(y+2, strings.TrimSpace(c.data.Branch), branchSize, false, true)
		y += 10
	}
	y, _ = c.center(y+8, "END OF SHIFT REPORT", c.data.BodySize-1, true, false)
	y += 8
	y, _ = c.center(y, fmt.Sprintf("Shift: #%d", report.ShiftID), c.data.BodySize-1, true, false)
	y += 8
	y = c.rule(y) + 10

	for _, line := range []string{
		"Cashier: " + fallbackReceiptValue(report.Cashier, "Not specified"),
		"Opened: " + report.OpenedAt,
		"Closed: " + report.ClosedAt,
	} {
		y, _ = c.left(y, line, c.data.BodySize-1, false)
		y += 8
	}

	y = c.reportSection(y, "SALES SUMMARY")
	y = c.reportRow(y, "First Invoice", fallbackReceiptValue(report.FirstInvoice, "-"), false)
	y = c.reportRow(y, "Last Invoice", fallbackReceiptValue(report.LastInvoice, "-"), false)
	y += 5
	y = c.reportRow(y, "Transactions", formatInt(report.Transactions), false)
	y = c.reportRow(y, "Items Sold", formatInt(report.ItemsSold), false)
	y = c.reportRow(y, "Returns", formatInt(report.Returns), false)
	y += 5
	y = c.reportRow(y, "Gross Sales", formatShiftUSD(report.GrossSales), false)
	y = c.reportRow(y, "Discounts", formatShiftUSD(-report.Discounts), false)
	if report.PriceDeductions != 0 {
		y = c.reportRow(y, "Price deductions", formatShiftUSD(-report.PriceDeductions), false)
	}
	if report.ReturnAmount != 0 {
		y = c.reportRow(y, "Return Value", formatShiftUSD(-report.ReturnAmount), false)
	}
	if report.CashRounding != 0 {
		y = c.reportRow(y, "Rounding", formatShiftUSD(report.CashRounding), false)
	}
	boxTop := c.rule(y) + 12
	y = boxTop + 8
	y, _ = c.twoCol(y, "NET SALES", formatShiftUSD(report.NetSales), c.data.BodySize+2, true)
	y += 8
	c.dottedBox(boxTop, y-boxTop)
	y += 6

	y = c.reportSection(y, "PAYMENT DETAILS")
	for _, payment := range report.Payments {
		label := "Received - " + fallbackReceiptValue(payment.AccountName, "Cash")
		if strings.EqualFold(strings.TrimSpace(payment.AccountName), "On Account") {
			label = "ON ACCOUNT (NOT COLLECTED)"
		} else if strings.EqualFold(strings.TrimSpace(payment.AccountName), "On Coupon") {
			label = "ON COUPON (AUTO-COUNTED)"
		}
		y, _ = c.twoCol(y, label, formatReceiptPayment(payment, report.RoundingMode), 10, true)
		y += 7
	}
	if len(report.Payments) == 0 {
		y = c.reportRow(y, "No payments", "-", false)
	}
	y = c.rule(y+3) + 12
	y, _ = c.twoCol(y, "PAID / EXPECTED", formatShiftUSD(report.TotalPayments), c.data.BodySize, true)
	y += 8
	y, _ = c.twoCol(y, "COUNTED TOTAL", formatShiftUSD(report.CountedTotal), c.data.BodySize, true)
	y += 8
	if report.CountedCoupon != 0 {
		y, _ = c.twoCol(y, "COUPON (AUTO-COUNTED)", formatShiftUSD(report.CountedCoupon), c.data.BodySize, true)
		y += 8
	}
	y, _ = c.twoCol(y, "DIFFERENCE", formatSignedShiftUSD(report.CountedTotal-report.TotalPayments), c.data.BodySize, true)
	y += 8
	if report.OnAccount > 0 {
		y, _ = c.twoCol(y, "ON ACCOUNT", formatShiftUSD(report.OnAccount), c.data.BodySize, true)
		y += 8
	}
	if len(report.Spend) > 0 {
		y = c.reportSection(y, "EXPENSES & LOANS")
		for _, line := range report.Spend {
			label := line.Label
			if line.Kind == "loan" {
				label = "Loan - " + label
			}
			y = c.reportRow(y, label, "-"+formatShiftSpendAmount(line.Amount, line.CurrencyCode), false)
		}
		y = c.rule(y+3) + 12
		y, _ = c.twoCol(y, "TOTAL OUT", formatShiftUSD(-report.SpendUSD), c.data.BodySize, true)
		y += 8
	}
	boxTop = c.rule(y) + 12
	y = boxTop + 8
	y, _ = c.twoCol(y, "NET TOTAL", formatShiftUSD(report.TotalPayments-report.SpendUSD), c.data.BodySize+2, true)
	y += 8
	c.dottedBox(boxTop, y-boxTop)
	y += 6
	y = c.reportSection(y, "CASH COUNT")
	// A method with nothing expected and nothing counted collapses to one
	// "None" line instead of three rows of zeros.
	if report.ExpectedUSD == 0 && report.CountedUSD == 0 && len(report.USDBillCounts) == 0 && report.OpeningFloatUSD == 0 && report.PaidOutUSD == 0 {
		y = c.reportRow(y, "USD", "None", true)
	} else {
		y, _ = c.left(y, "USD", c.data.BodySize-1, true)
		y += 7
		for _, bill := range report.USDBillCounts {
			label := fmt.Sprintf("$%d x %s", bill.Denomination, formatInt(bill.Count))
			value := formatShiftUSD(bill.Denomination * bill.Count * 100)
			y = c.reportRow(y, label, value, false)
		}
		if len(report.USDBillCounts) > 0 {
			y += 4
		}
		if report.OpeningFloatUSD != 0 {
			y = c.reportRow(y, "Opening float", formatShiftUSD(report.OpeningFloatUSD), false)
		}
		if report.PaidOutUSD != 0 {
			y = c.reportRow(y, "Paid out", formatShiftUSD(-report.PaidOutUSD), false)
		}
		y = c.reportRow(y, "Expected", formatShiftUSD(report.ExpectedUSD), false)
		y = c.reportRow(y, "Counted", formatShiftUSD(report.CountedUSD), false)
		y = c.reportRow(y, "Difference", formatSignedShiftUSD(report.CountedUSD-report.ExpectedUSD), true)
	}
	y += 7
	if report.ExpectedLBP == 0 && report.CountedLBP == 0 && report.OpeningFloatLBP == 0 && report.PaidOutLBP == 0 {
		y = c.reportRow(y, "LBP", "None", true)
	} else {
		y, _ = c.left(y, "LBP", c.data.BodySize-1, true)
		y += 7
		if report.OpeningFloatLBP != 0 {
			y = c.reportRow(y, "Opening float", formatShiftLBP(report.OpeningFloatLBP), false)
		}
		if report.PaidOutLBP != 0 {
			y = c.reportRow(y, "Paid out", formatShiftLBP(-report.PaidOutLBP), false)
		}
		y = c.reportRow(y, "Expected", formatShiftLBP(report.ExpectedLBP), false)
		y = c.reportRow(y, "Counted", formatShiftLBP(report.CountedLBP), false)
		y = c.reportRow(y, "Difference", formatSignedShiftLBP(report.CountedLBP-report.ExpectedLBP), true)
	}
	y += 7
	y = c.reportUSDCount(y, "VISA", report.ExpectedVisa, report.CountedVisa)
	y += 7
	y = c.reportUSDCount(y, "WHISH", report.ExpectedWhish, report.CountedWhish)

	y = c.rule(y+9) + 10
	if report.SignatureLines {
		y += 12
		y, _ = c.left(y, "Cashier: ____________________", c.data.BodySize-1, false)
		y += 20
		y, _ = c.left(y, "Manager: ____________________", c.data.BodySize-1, false)
	}
	y += 50
	return y, nil
}

// reportUSDCount prints a USD-denominated method's expected/counted block, or
// a single "None" line when both are zero.
func (c *receiptCanvas) reportUSDCount(y int, title string, expected, counted int64) int {
	if expected == 0 && counted == 0 {
		return c.reportRow(y, title, "None", true)
	}
	y, _ = c.left(y, title, c.data.BodySize-1, true)
	y += 7
	y = c.reportRow(y, "Expected", formatShiftUSD(expected), false)
	y = c.reportRow(y, "Counted", formatShiftUSD(counted), false)
	return c.reportRow(y, "Difference", formatSignedShiftUSD(counted-expected), true)
}

func (c *receiptCanvas) reportSection(y int, title string) int {
	y = c.rule(y) + 10
	y, _ = c.center(y, title, c.data.BodySize-1, true, false)
	return c.rule(y+6) + 10
}

func (c *receiptCanvas) reportRow(y int, label, value string, bold bool) int {
	y, _ = c.twoCol(y, label, value, c.data.BodySize-1, bold)
	return y + 7
}

func formatShiftUSD(cents int64) string {
	if cents < 0 {
		return "-$" + fmt.Sprintf("%.2f", float64(-cents)/100)
	}
	return "$" + fmt.Sprintf("%.2f", float64(cents)/100)
}

func formatSignedShiftUSD(cents int64) string {
	if cents > 0 {
		return "+" + formatShiftUSD(cents)
	}
	return formatShiftUSD(cents)
}

// formatShiftSpendAmount prints an expense/loan amount (x100, in its own
// currency) the way the rest of the report prints that currency.
func formatShiftSpendAmount(amount int64, currencyCode string) string {
	switch strings.ToUpper(strings.TrimSpace(currencyCode)) {
	case "USD":
		return formatShiftUSD(amount)
	case "LBP":
		return formatShiftLBP(amount / 100)
	}
	return fmt.Sprintf("%.2f %s", float64(amount)/100, strings.ToUpper(strings.TrimSpace(currencyCode)))
}

func formatShiftLBP(amount int64) string {
	return formatInt(amount) + " LBP"
}

func formatSignedShiftLBP(amount int64) string {
	if amount > 0 {
		return "+" + formatShiftLBP(amount)
	}
	return formatShiftLBP(amount)
}
