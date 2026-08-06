package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

const autoCloseStaleShiftsFlag = "auto_close_stale_shifts"

func toLocalTime(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}
	return s
}

func shiftToMap(s db.Shift) map[string]any {
	return map[string]any{
		"id":                    s.ID,
		"isClosed":              s.IsClosed,
		"createdAt":             toLocalTime(s.CreatedAt),
		"closedAt":              toLocalTime(s.ClosedAt),
		"closedBySalespersonId": s.ClosedBySalespersonID,
		"countedUsd":            s.CountedUsd,
		"countedLbp":            s.CountedLbp,
		"countedVisa":           s.CountedVisa,
		"countedWhish":          s.CountedWhish,
		"countedUsdBreakdown":   s.CountedUsdBreakdown,
	}
}

func (a *App) OpenShift() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	if err := a.store.CloseOpenShifts(a.ctx, now); err != nil {
		return nil, fmt.Errorf("close previous shift: %w", err)
	}

	shift, err := a.store.CreateShift(a.ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("create shift: %w", err)
	}

	return shiftToMap(shift), nil
}

func (a *App) GetOpenShift() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("no open shift: %w", err)
	}

	return shiftToMap(shift), nil
}

// closeStaleOpenShifts closes open shifts created on an earlier local calendar
// day. Physical counts and closing person are unavailable during startup.
func (a *App) closeStaleOpenShifts() error {
	enabled, err := a.store.GetFeatureFlag(a.ctx, autoCloseStaleShiftsFlag)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if enabled == 0 {
		return nil
	}
	return a.store.CloseStaleOpenShifts(a.ctx, time.Now().Format("2006-01-02 15:04:05"))
}

func (a *App) SetAutoCloseStaleShiftsEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: autoCloseStaleShiftsFlag, Enabled: boolInt(enabled),
	})
}

func (a *App) CloseShiftByID(id, closingPersonID, countedLBP, countedVisa, countedWhish int64, usdBreakdownJSON string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if id <= 0 {
		return nil, fmt.Errorf("invalid shift")
	}
	if closingPersonID <= 0 {
		return nil, fmt.Errorf("closing person is required")
	}
	if countedLBP < 0 {
		return nil, fmt.Errorf("counted LBP cannot be negative")
	}
	if countedVisa < 0 || countedWhish < 0 {
		return nil, fmt.Errorf("counted Visa and Whish cannot be negative")
	}
	usdBillCounts, countedUSD, canonicalBreakdown, err := parseUSDBillCounts(usdBreakdownJSON)
	if err != nil {
		return nil, err
	}

	closedAt := time.Now().Format("2006-01-02 15:04:05")
	printData, receiptEnabled, err := a.buildShiftClosePrintData(
		id, closingPersonID, countedUSD, countedLBP, countedVisa, countedWhish, usdBillCounts, closedAt,
		true,
	)
	if err != nil {
		return nil, err
	}

	updated, err := a.store.CloseShift(a.ctx, db.CloseShiftParams{
		ClosedAt:              closedAt,
		ClosedBySalespersonID: closingPersonID,
		CountedUsd:            countedUSD,
		CountedLbp:            countedLBP,
		CountedVisa:           countedVisa,
		CountedWhish:          countedWhish,
		CountedUsdBreakdown:   canonicalBreakdown,
		ID:                    id,
	})
	if err != nil {
		return nil, fmt.Errorf("close shift: %w", err)
	}
	if updated != 1 {
		return nil, fmt.Errorf("shift is already closed or does not exist")
	}

	result := map[string]any{"receiptPrinted": false}
	if !receiptEnabled {
		return result, nil
	}
	if err := printReceiptToPrinter(printData); err != nil {
		result["receiptError"] = err.Error()
		return result, nil
	}
	result["receiptPrinted"] = true
	return result, nil
}

// PrintCurrentCloseShiftSample prints the current open shift's report without
// changing the shift or creating a new one. Counted cash is intentionally zero
// because the physical cash count is not known until the shift is closed.
func (a *App) PrintCurrentCloseShiftSample() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("open shift: %w", err)
	}
	salespersons, err := a.store.ListSalespersons(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("salespersons: %w", err)
	}
	if len(salespersons) == 0 {
		return nil, fmt.Errorf("at least one salesperson is required to print a shift sample")
	}

	printData, _, err := a.buildShiftClosePrintData(
		shift.ID, salespersons[0].ID, 0, 0, 0, 0, nil,
		time.Now().Format("2006-01-02 15:04:05"), true,
	)
	if err != nil {
		return nil, err
	}
	if err := printReceiptToPrinter(printData); err != nil {
		return nil, fmt.Errorf("print close shift sample: %w", err)
	}
	return map[string]any{"receiptPrinted": true}, nil
}

// PrintClosedShiftReceipt reprints the end-of-shift receipt for an existing
// closed shift without changing any of its stored totals.
func (a *App) PrintClosedShiftReceipt(shiftID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if shiftID <= 0 {
		return nil, fmt.Errorf("invalid shift")
	}
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return nil, fmt.Errorf("shift: %w", err)
	}
	if shift.IsClosed == 0 {
		return nil, fmt.Errorf("shift is still open")
	}
	breakdownJSON := strings.TrimSpace(shift.CountedUsdBreakdown)
	if breakdownJSON == "" {
		breakdownJSON = "{}"
	}
	usdBillCounts, _, _, err := parseUSDBillCounts(breakdownJSON)
	if err != nil {
		return nil, err
	}
	printData, receiptEnabled, err := a.buildShiftClosePrintData(
		shift.ID, shift.ClosedBySalespersonID, shift.CountedUsd, shift.CountedLbp, shift.CountedVisa, shift.CountedWhish,
		usdBillCounts, shift.ClosedAt, false,
	)
	if err != nil {
		return nil, err
	}
	if !receiptEnabled {
		return map[string]any{"receiptPrinted": false}, nil
	}
	if err := printReceiptToPrinter(printData); err != nil {
		return nil, fmt.Errorf("print close shift receipt: %w", err)
	}
	return map[string]any{"receiptPrinted": true}, nil
}

func (a *App) buildShiftClosePrintData(
	shiftID, closingPersonID, countedUSD, countedLBP, countedVisa, countedWhish int64,
	usdBillCounts []shiftDenominationCount,
	closedAt string,
	requireOpen bool,
) (printReceiptData, bool, error) {
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("shift: %w", err)
	}
	if requireOpen && shift.IsClosed != 0 {
		return printReceiptData{}, false, fmt.Errorf("shift is already closed")
	}
	closingPersonName := "Unknown"
	if closingPersonID > 0 {
		closingPerson, err := a.store.GetSalespersonByID(a.ctx, closingPersonID)
		if err != nil {
			return printReceiptData{}, false, fmt.Errorf("closing person: %w", err)
		}
		closingPersonName = closingPerson.FullName
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("settings: %w", err)
	}
	summary, err := a.store.GetShiftReportSummary(a.ctx, shiftID)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("shift report summary: %w", err)
	}
	paymentRows, err := a.store.ListShiftPaymentTotals(a.ctx, shiftID)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("shift payment totals: %w", err)
	}
	invoicePaymentTotal, err := a.store.GetShiftInvoicePaymentTotal(a.ctx, shiftID)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("shift invoice payment total: %w", err)
	}

	payments := make([]receiptPayment, 0, len(paymentRows))
	var totalPayments, onAccount, countedCoupon int64
	var expectedUSD, expectedLBPCents, expectedVisa, expectedWhish int64
	for _, row := range paymentRows {
		total := databaseInt64(row.Total)
		if strings.EqualFold(strings.TrimSpace(row.AccountName), "On Account") {
			onAccount += total
		} else {
			totalPayments += total
		}
		if strings.EqualFold(strings.TrimSpace(row.AccountName), "On Coupon") {
			countedCoupon += total
		}
		payments = append(payments, receiptPayment{
			AccountID:    row.AccountID,
			AccountName:  row.AccountName,
			CurrencyCode: strings.ToUpper(strings.TrimSpace(row.CurrencyCode)),
			AmountCents:  total,
			EntryType:    "payment",
		})
		accountName := strings.TrimSpace(row.AccountName)
		if strings.EqualFold(accountName, "Visa") && strings.EqualFold(row.CurrencyCode, "USD") {
			expectedVisa += total
			continue
		}
		if strings.EqualFold(accountName, "Whish") && strings.EqualFold(row.CurrencyCode, "USD") {
			expectedWhish += total
			continue
		}
		if !isCashAccount(row.AccountName, row.CurrencyCode) {
			continue
		}
		if strings.EqualFold(row.CurrencyCode, "LBP") {
			expectedLBPCents += total
		} else if strings.EqualFold(row.CurrencyCode, "USD") {
			expectedUSD += total
		}
	}

	rate := settings.ExchangeRate
	if rate <= 0 {
		rate = 90000
	}
	expectedLBP, roundErr := roundLBPFromCents(expectedLBPCents, rate, settings.RoundingMode)
	if roundErr != nil {
		expectedLBP = expectedLBPCents * rate / 100
	}
	countedTotal, err := reconcileShiftCountedTotal(
		totalPayments,
		expectedUSD, countedUSD,
		expectedLBP, countedLBP,
		expectedVisa, countedVisa,
		expectedWhish, countedWhish,
		rate, settings.RoundingMode,
	)
	if err != nil {
		return printReceiptData{}, false, err
	}
	overpayment := databaseInt64(invoicePaymentTotal) - databaseInt64(summary.NetSales)
	if overpayment < 0 {
		overpayment = 0
	}
	report := &shiftReportData{
		ShiftID:         shift.ID,
		Cashier:         closingPersonName,
		Register:        strings.TrimSpace(settings.CounterNumber),
		OpenedAt:        formatShiftReportTime(shift.CreatedAt),
		ClosedAt:        formatShiftReportTime(closedAt),
		FirstInvoice:    databaseString(summary.FirstInvoice),
		LastInvoice:     databaseString(summary.LastInvoice),
		Transactions:    summary.Transactions,
		ItemsSold:       databaseInt64(summary.ItemsSold),
		Returns:         summary.Returns,
		GrossSales:      databaseInt64(summary.GrossSales),
		Discounts:       databaseInt64(summary.Discounts),
		PriceDeductions: databaseInt64(summary.PriceDeductions),
		ReturnAmount:    databaseInt64(summary.ReturnAmount),
		Tax:             databaseInt64(summary.Tax),
		NetSales:        databaseInt64(summary.NetSales),
		TotalPayments:   totalPayments,
		CountedTotal:    countedTotal,
		CountedCoupon:   countedCoupon,
		OnAccount:       onAccount,
		Overpayment:     overpayment,
		Payments:        payments,
		ExpectedUSD:     expectedUSD,
		CountedUSD:      countedUSD,
		USDBillCounts:   usdBillCounts,
		ExpectedLBP:     expectedLBP,
		CountedLBP:      countedLBP,
		ExpectedVisa:    expectedVisa,
		CountedVisa:     countedVisa,
		ExpectedWhish:   expectedWhish,
		CountedWhish:    countedWhish,
		ExchangeRate:    rate,
		RoundingMode:    settings.RoundingMode,
	}
	marketName := strings.TrimSpace(settings.MarketName)
	if marketName == "" {
		marketName = "Market"
	}

	return printReceiptData{
		StoreName:     marketName,
		Branch:        settings.BranchName,
		Phone:         settings.MarketPhone,
		BranchPhone:   settings.BranchPhone,
		PrinterName:   settings.ReceiptPrinter,
		Font:          settings.ReceiptFont,
		BodyFont:      settings.ReceiptBodyFont,
		TitleSize:     settings.ReceiptTitleSize,
		BodySize:      settings.ReceiptBodySize,
		Cutoff:        settings.ReceiptCutoff != 0,
		ReceiptWidth:  settings.ReceiptWidth,
		ReceiptHeight: settings.ReceiptHeight,
		ShiftReport:   report,
	}, settings.ReceiptEnabled != 0, nil
}

func sumShiftPaymentTotals(rows []db.ListShiftPaymentTotalsRow) int64 {
	var total int64
	for _, row := range rows {
		total += databaseInt64(row.Total)
	}
	return total
}

func reconcileShiftCountedTotal(
	paidTotal,
	expectedUSD, countedUSD,
	expectedLBP, countedLBP,
	expectedVisa, countedVisa,
	expectedWhish, countedWhish,
	exchangeRate int64,
	roundingMode string,
) (int64, error) {
	lbpDifference := countedLBP - expectedLBP
	absLBPDifference := lbpDifference
	if absLBPDifference < 0 {
		absLBPDifference = -absLBPDifference
	}
	lbpDifferenceCents, err := roundCentsFromLBP(absLBPDifference, exchangeRate, roundingMode)
	if err != nil {
		return 0, fmt.Errorf("counted LBP difference: %w", err)
	}
	if lbpDifference < 0 {
		lbpDifferenceCents = -lbpDifferenceCents
	}
	return paidTotal +
		(countedUSD - expectedUSD) +
		lbpDifferenceCents +
		(countedVisa - expectedVisa) +
		(countedWhish - expectedWhish), nil
}

var usdBillDenominations = []int64{100, 50, 20, 10, 5, 1}

func parseUSDBillCounts(raw string) ([]shiftDenominationCount, int64, string, error) {
	var counts map[string]int64
	if err := json.Unmarshal([]byte(raw), &counts); err != nil {
		return nil, 0, "", fmt.Errorf("invalid USD bill counts")
	}
	allowed := make(map[string]int64, len(usdBillDenominations))
	for _, denomination := range usdBillDenominations {
		allowed[strconv.FormatInt(denomination, 10)] = denomination
	}
	for key, count := range counts {
		if _, ok := allowed[key]; !ok {
			return nil, 0, "", fmt.Errorf("unsupported USD denomination %q", key)
		}
		if count < 0 {
			return nil, 0, "", fmt.Errorf("USD bill counts cannot be negative")
		}
	}

	normalized := make(map[string]int64, len(usdBillDenominations))
	breakdown := make([]shiftDenominationCount, 0, len(usdBillDenominations))
	var totalCents int64
	for _, denomination := range usdBillDenominations {
		key := strconv.FormatInt(denomination, 10)
		count := counts[key]
		normalized[key] = count
		if count == 0 {
			continue
		}
		if count > math.MaxInt64/denomination/100 {
			return nil, 0, "", fmt.Errorf("USD bill count is too large")
		}
		value := denomination * count * 100
		if totalCents > math.MaxInt64-value {
			return nil, 0, "", fmt.Errorf("counted USD total is too large")
		}
		totalCents += value
		breakdown = append(breakdown, shiftDenominationCount{
			Denomination: denomination,
			Count:        count,
		})
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return nil, 0, "", fmt.Errorf("encode USD bill counts: %w", err)
	}
	return breakdown, totalCents, string(canonical), nil
}

func isCashAccount(name, currencyCode string) bool {
	normalizedName := strings.ToUpper(strings.TrimSpace(name))
	normalizedCurrency := strings.ToUpper(strings.TrimSpace(currencyCode))
	return strings.Contains(normalizedName, "CASH") ||
		(normalizedCurrency != "" && normalizedName == normalizedCurrency)
}

func databaseString(value any) string {
	switch text := value.(type) {
	case string:
		return text
	case []byte:
		return string(text)
	default:
		return ""
	}
}

func formatShiftReportTime(value string) string {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.Format("2006-01-02 15:04")
		}
	}
	return value
}
