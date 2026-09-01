package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

const autoCloseStaleShiftsFlag = "auto_close_stale_shifts"

// openingFloatEnabledFlag is the master switch for the opening cash float
// feature. Default (missing row or 1) = enabled, the historical behaviour.
// Set to 0 and there is no opening float anywhere: new shifts capture none,
// the Shifts page hides the "Set opening cash" controls, and every
// downstream total, receipt and balance ignores it (countOpeningCashOnClose
// forces false, which the close reconciliation and openingFloatCentsForShift
// already key off).
const openingFloatEnabledFlag = "opening_float_enabled"

// countOpeningCashOnCloseFlag controls whether the physical cash counted at
// close-shift is expected to include the opening float. Default (missing row
// or 1) keeps the historical behaviour: the cashier counts the whole drawer,
// opening float included. Set to 0 to have the cashier enter only the cash
// taken in during the shift - the opening float is then left out of both the
// expected total and the printed reconciliation so the variance still lines
// up.
const countOpeningCashOnCloseFlag = "count_opening_cash_on_close"

// openingFloatEnabled reports whether the opening cash float feature is
// switched on at all, defaulting to true when the flag has never been set.
func (a *App) openingFloatEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, openingFloatEnabledFlag); err == nil {
		return v != 0
	}
	return true
}

// countOpeningCashOnClose reports the effective value of
// countOpeningCashOnCloseFlag, defaulting to true when the flag has never
// been set. Always false when the opening float feature is disabled entirely.
func (a *App) countOpeningCashOnClose() bool {
	if !a.openingFloatEnabled() {
		return false
	}
	if v, err := a.store.GetFeatureFlag(a.ctx, countOpeningCashOnCloseFlag); err == nil {
		return v != 0
	}
	return true
}

// errShiftAlreadyClosed is returned from inside CloseShiftByID's write
// transaction when CloseShift updates zero rows, so the outbox insert is
// rolled back with it rather than committing a sync for a close that never
// happened.
var errShiftAlreadyClosed = errors.New("shift already closed or missing")

func shiftToMap(s db.Shift) map[string]any {
	return map[string]any{
		"id":                    s.ID,
		"isClosed":              s.IsClosed,
		"createdAt":             s.CreatedAt,
		"closedAt":              s.ClosedAt,
		"closedBySalespersonId": s.ClosedBySalespersonID,
		"countedUsd":            s.CountedUsd,
		"countedLbp":            s.CountedLbp,
		"countedVisa":           s.CountedVisa,
		"countedWhish":          s.CountedWhish,
		"countedUsdBreakdown":   s.CountedUsdBreakdown,
		"openingFloatUsd":       s.OpeningFloatUsd,
		"openingFloatLbp":       s.OpeningFloatLbp,
	}
}

func (a *App) OpenShift() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if err := a.store.CloseOpenShifts(a.ctx, now); err != nil {
		return nil, fmt.Errorf("close previous shift: %w", err)
	}

	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	openingFloatUsd, openingFloatLbp := settings.DefaultOpeningFloatUsd, settings.DefaultOpeningFloatLbp
	if !a.openingFloatEnabled() {
		openingFloatUsd, openingFloatLbp = 0, 0
	}
	shift, err := a.store.CreateShift(a.ctx, db.CreateShiftParams{
		OpeningFloatUsd: openingFloatUsd,
		OpeningFloatLbp: openingFloatLbp,
	})
	if err != nil {
		return nil, fmt.Errorf("create shift: %w", err)
	}

	return shiftToMap(shift), nil
}

// UpdateDefaultOpeningFloat sets the opening cash float applied to every
// shift opened from now on. It does not affect the currently open shift -
// use ApplyOpeningFloatToCurrentShift for that.
func (a *App) UpdateDefaultOpeningFloat(usdCents, lbpUnits int64, managerPassword string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.openingFloatEnabled() {
		return fmt.Errorf("opening cash float is disabled")
	}
	if usdCents < 0 || lbpUnits < 0 {
		return fmt.Errorf("opening float cannot be negative")
	}
	if err := a.verifyManagerPassword(managerPassword); err != nil {
		return err
	}
	return a.store.UpdateDefaultOpeningFloat(a.ctx, db.UpdateDefaultOpeningFloatParams{
		DefaultOpeningFloatUsd: usdCents,
		DefaultOpeningFloatLbp: lbpUnits,
	})
}

// ApplyOpeningFloatToCurrentShift updates the opening float on the shift
// that's currently open, taking effect immediately rather than waiting for
// the next OpenShift.
func (a *App) ApplyOpeningFloatToCurrentShift(usdCents, lbpUnits int64, managerPassword string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !a.openingFloatEnabled() {
		return nil, fmt.Errorf("opening cash float is disabled")
	}
	if usdCents < 0 || lbpUnits < 0 {
		return nil, fmt.Errorf("opening float cannot be negative")
	}
	if err := a.verifyManagerPassword(managerPassword); err != nil {
		return nil, err
	}
	openShift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("no open shift: %w", err)
	}
	shift, err := a.store.UpdateShiftOpeningFloat(a.ctx, db.UpdateShiftOpeningFloatParams{
		OpeningFloatUsd: usdCents,
		OpeningFloatLbp: lbpUnits,
		ID:              openShift.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("apply opening float: %w", err)
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
	return a.store.CloseStaleOpenShifts(a.ctx, time.Now().UTC().Format("2006-01-02 15:04:05"))
}

func (a *App) SetAutoCloseStaleShiftsEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: autoCloseStaleShiftsFlag, Enabled: boolInt(enabled),
	})
}

// SetOpeningFloatEnabled is the master on/off switch for the opening cash
// float feature (see openingFloatEnabledFlag).
func (a *App) SetOpeningFloatEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: openingFloatEnabledFlag, Enabled: boolInt(enabled),
	})
}

// SetCountOpeningCashOnCloseEnabled toggles whether the close-shift cash
// count is expected to include the opening float (see
// countOpeningCashOnCloseFlag).
func (a *App) SetCountOpeningCashOnCloseEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: countOpeningCashOnCloseFlag, Enabled: boolInt(enabled),
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

	closedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	printData, receiptEnabled, err := a.buildShiftClosePrintData(
		id, closingPersonID, countedUSD, countedLBP, countedVisa, countedWhish, usdBillCounts, closedAt,
		true,
	)
	if err != nil {
		return nil, err
	}

	var expectedUSD, expectedLBP, expectedVisa, expectedWhish int64
	if printData.ShiftReport != nil {
		expectedUSD = printData.ShiftReport.ExpectedUSD
		expectedLBP = printData.ShiftReport.ExpectedLBP
		expectedVisa = printData.ShiftReport.ExpectedVisa
		expectedWhish = printData.ShiftReport.ExpectedWhish
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}

	// Snapshot the still-open shift and closing person for the sync payload
	// before the close runs — CreatedAt and the opening floats don't change
	// on close, and reading them here keeps the transaction below to just
	// the close plus the outbox insert.
	var openedAt time.Time
	var openingFloatUsd, openingFloatLbp int64
	var closingPersonName string
	if centralURL != "" {
		shift, shiftErr := a.store.GetShiftByID(a.ctx, id)
		if shiftErr != nil {
			return nil, fmt.Errorf("load shift for sync: %w", shiftErr)
		}
		openedAt = parseShiftTimestamp(shift.CreatedAt)
		openingFloatUsd, openingFloatLbp = shift.OpeningFloatUsd, shift.OpeningFloatLbp
		if cp, cpErr := a.store.GetSalespersonByID(a.ctx, closingPersonID); cpErr == nil {
			closingPersonName = cp.FullName
		}
	}

	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		updated, err := q.CloseShift(a.ctx, db.CloseShiftParams{
			ClosedAt:              closedAt,
			ClosedBySalespersonID: closingPersonID,
			CountedUsd:            countedUSD,
			CountedLbp:            countedLBP,
			CountedVisa:           countedVisa,
			CountedWhish:          countedWhish,
			CountedUsdBreakdown:   canonicalBreakdown,
			ExpectedUsd:           expectedUSD,
			ExpectedLbp:           expectedLBP,
			ExpectedVisa:          expectedVisa,
			ExpectedWhish:         expectedWhish,
			ID:                    id,
		})
		if err != nil {
			return fmt.Errorf("close shift: %w", err)
		}
		if updated != 1 {
			return errShiftAlreadyClosed
		}
		return queueBranchShiftCloseSync(a.ctx, q, centralURL, branchShiftClosePayload{
			ShiftRef:          fmt.Sprintf("shift-%d", id),
			BranchShiftID:     id,
			OpenedAt:          openedAt,
			ClosedAt:          parseShiftTimestamp(closedAt),
			ClosingPersonName: closingPersonName,
			OpeningFloatUsd:   openingFloatUsd,
			OpeningFloatLbp:   openingFloatLbp,
			ExpectedUsd:       expectedUSD,
			ExpectedLbp:       expectedLBP,
			ExpectedVisa:      expectedVisa,
			ExpectedWhish:     expectedWhish,
			CountedUsd:        countedUSD,
			CountedLbp:        countedLBP,
			CountedVisa:       countedVisa,
			CountedWhish:      countedWhish,
			OccurredAt:        time.Now().UTC(),
		})
	})
	if err != nil {
		if errors.Is(err, errShiftAlreadyClosed) {
			return nil, fmt.Errorf("shift is already closed or does not exist")
		}
		return nil, err
	}
	if centralURL != "" {
		a.kickSyncWorker()
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
		time.Now().UTC().Format("2006-01-02 15:04:05"), true,
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

// computeExpectedShiftTotals computes expected USD/LBP/Visa/Whish for a
// shift - including its opening float - purely from live transaction data.
// It doesn't require or depend on any counted values, so it's safe to call
// before the cashier has counted anything (used by GetShiftExpectedTotals
// to prefill the close-shift dialog) as well as from buildShiftClosePrintData.
func (a *App) computeExpectedShiftTotals(shiftID int64) (expectedUSD, expectedLBP, expectedVisa, expectedWhish int64, err error) {
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("shift: %w", err)
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("settings: %w", err)
	}
	paymentRows, err := a.incomeAccountTotals(shiftID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("shift payment totals: %w", err)
	}

	var expectedLBPCents int64
	for _, row := range paymentRows {
		total := databaseInt64(row.Total)
		accountName := strings.TrimSpace(row.Name)
		if strings.EqualFold(accountName, "Visa") && strings.EqualFold(row.CurrencyCode, "USD") {
			expectedVisa += total
			continue
		}
		if strings.EqualFold(accountName, "Whish") && strings.EqualFold(row.CurrencyCode, "USD") {
			expectedWhish += total
			continue
		}
		if !isCashAccount(row.Name, row.CurrencyCode) {
			continue
		}
		if strings.EqualFold(row.CurrencyCode, "LBP") {
			expectedLBPCents += total
		} else if strings.EqualFold(row.CurrencyCode, "USD") {
			expectedUSD += total
		}
	}

	lbpCurrency, err := resolveCurrency(a.ctx, a.store.Queries, "LBP")
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("LBP currency: %w", err)
	}

	// The opening float is cash that was already in the drawer at shift
	// start, not a transaction - fold it into expected so a shift that
	// refunds more than it sold doesn't show a false overage just because
	// counted cash can't go negative. When countOpeningCashOnCloseFlag is
	// off the cashier counts only the shift's own cash, so the float is
	// left out of expected to match.
	openingFloatLBPCents, err := roundCentsFromCurrencyUnits(shift.OpeningFloatLbp, lbpCurrency.UnitsPerUsdMicros, settings.RoundingMode)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("opening float LBP: %w", err)
	}
	if a.countOpeningCashOnClose() {
		expectedUSD += shift.OpeningFloatUsd
		expectedLBPCents += openingFloatLBPCents
	}

	// expectedLBPCents can be negative when this shift refunded more LBP than
	// it took in (and the opening float isn't folded in). roundCurrencyUnitsFromCents
	// only accepts a non-negative amount, so round the magnitude and re-apply
	// the sign - the reconciliation math downstream already handles a negative
	// expected LBP.
	lbpSign, lbpCents := int64(1), expectedLBPCents
	if lbpCents < 0 {
		lbpSign, lbpCents = -1, -lbpCents
	}
	expectedLBP, err = roundCurrencyUnitsFromCents(lbpCents, lbpCurrency.UnitsPerUsdMicros, lbpCurrency.CashRoundingUnit, settings.RoundingMode)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("expected LBP: %w", err)
	}
	expectedLBP *= lbpSign
	return expectedUSD, expectedLBP, expectedVisa, expectedWhish, nil
}

// suggestUSDBillBreakdown greedily splits a USD cents amount into whole
// bills across the denominations this app counts, largest first. It's only
// a starting suggestion for the close-shift dialog - the actual drawer
// might hold a different mix of bills for the same total, so the cashier
// must still recount and can freely edit every denomination.
func suggestUSDBillBreakdown(cents int64) []shiftDenominationCount {
	dollars := cents / 100
	if dollars <= 0 {
		return nil
	}
	breakdown := make([]shiftDenominationCount, 0, len(usdBillDenominations))
	remaining := dollars
	for _, denom := range usdBillDenominations {
		count := remaining / denom
		if count <= 0 {
			continue
		}
		breakdown = append(breakdown, shiftDenominationCount{Denomination: denom, Count: count})
		remaining -= count * denom
	}
	return breakdown
}

// GetShiftExpectedTotals returns the shift's current expected totals and a
// suggested USD bill breakdown, computed live from transactions recorded so
// far - used to prefill the close-shift dialog before the cashier counts.
func (a *App) GetShiftExpectedTotals(shiftID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	expectedUSD, expectedLBP, expectedVisa, expectedWhish, err := a.computeExpectedShiftTotals(shiftID)
	if err != nil {
		return nil, err
	}
	breakdown := suggestUSDBillBreakdown(expectedUSD)
	usdBillCounts := make(map[string]int64, len(breakdown))
	for _, bill := range breakdown {
		usdBillCounts[strconv.FormatInt(bill.Denomination, 10)] = bill.Count
	}
	return map[string]any{
		"expectedUsd":   expectedUSD,
		"expectedLbp":   expectedLBP,
		"expectedVisa":  expectedVisa,
		"expectedWhish": expectedWhish,
		"usdBillCounts": usdBillCounts,
	}, nil
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
	paymentRows, err := a.incomeAccountTotals(shiftID)
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("shift payment totals: %w", err)
	}

	payments := make([]receiptPayment, 0, len(paymentRows))
	currencyCache := make(map[string]receiptCurrency)
	var totalPayments, onAccount, countedCoupon int64
	for _, row := range paymentRows {
		total := databaseInt64(row.Total)
		if isNonCollectedIncomeAccount(row.Name) {
			onAccount += total
		} else {
			totalPayments += total
		}
		if strings.EqualFold(strings.TrimSpace(row.Name), "On Coupon") {
			countedCoupon += total
		}
		rowCurrency, cached := currencyCache[row.CurrencyCode]
		if !cached {
			if resolved, currErr := resolveCurrency(a.ctx, a.store.Queries, row.CurrencyCode); currErr == nil {
				rowCurrency = toReceiptCurrency(resolved)
			}
			currencyCache[row.CurrencyCode] = rowCurrency
		}
		payments = append(payments, receiptPayment{
			AccountID:    row.ID,
			AccountName:  row.Name,
			CurrencyCode: strings.ToUpper(strings.TrimSpace(row.CurrencyCode)),
			Currency:     rowCurrency,
			AmountCents:  total,
			EntryType:    "payment",
		})
	}

	// Close-shift reconciliation still buckets by the specific categories
	// USD-cash/LBP/Visa/Whish rather than arbitrary currencies - this is a
	// payment-method-category reconciliation report, not currency rounding,
	// and is out of scope for the currency generalization.
	lbpCurrency, err := resolveCurrency(a.ctx, a.store.Queries, "LBP")
	if err != nil {
		return printReceiptData{}, false, fmt.Errorf("LBP currency: %w", err)
	}
	rate := lbpCurrency.UnitsPerUsdMicros / unitsPerUsdMicroScale

	expectedUSD, expectedLBP, expectedVisa, expectedWhish, err := a.computeExpectedShiftTotals(shiftID)
	if err != nil {
		return printReceiptData{}, false, err
	}
	countedTotal, err := reconcileShiftCountedTotal(
		totalPayments,
		expectedUSD, countedUSD,
		expectedLBP, countedLBP,
		expectedVisa, countedVisa,
		expectedWhish, countedWhish,
		lbpCurrency, settings.RoundingMode,
	)
	if err != nil {
		return printReceiptData{}, false, err
	}
	// When the opening float isn't counted at close, keep it off the printed
	// reconciliation too - the expected totals already exclude it (see
	// computeExpectedShiftTotals), so showing the float row would just make
	// the arithmetic on the receipt look inconsistent.
	reportOpeningFloatUSD, reportOpeningFloatLBP := shift.OpeningFloatUsd, shift.OpeningFloatLbp
	if !a.countOpeningCashOnClose() {
		reportOpeningFloatUSD, reportOpeningFloatLBP = 0, 0
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
		Payments:        payments,
		OpeningFloatUSD: reportOpeningFloatUSD,
		ExpectedUSD:     expectedUSD,
		CountedUSD:      countedUSD,
		USDBillCounts:   usdBillCounts,
		OpeningFloatLBP: reportOpeningFloatLBP,
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

func reconcileShiftCountedTotal(
	paidTotal,
	expectedUSD, countedUSD,
	expectedLBP, countedLBP,
	expectedVisa, countedVisa,
	expectedWhish, countedWhish int64,
	lbpCurrency db.Currency,
	roundingMode string,
) (int64, error) {
	lbpDifference := countedLBP - expectedLBP
	absLBPDifference := lbpDifference
	if absLBPDifference < 0 {
		absLBPDifference = -absLBPDifference
	}
	lbpDifferenceCents, err := roundCentsFromCurrencyUnits(absLBPDifference, lbpCurrency.UnitsPerUsdMicros, roundingMode)
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

// formatShiftReportTime renders a stored UTC timestamp in the local
// timezone for the printed shift receipt - the only place shift times are
// still shown pre-formatted rather than left for the frontend to convert.
func formatShiftReportTime(value string) string {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.In(time.Local).Format("2006-01-02 15:04")
		}
	}
	return value
}

// parseShiftTimestamp reads one of kashi-pos's stored timestamp string
// formats (all UTC — see migration 000066) into a time.Time for the sync
// payload. A value it can't parse yields the zero time, which kashi treats
// as "unknown" rather than rejecting the close.
func parseShiftTimestamp(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
