package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

func (a *App) ListInventories() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListInventories(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, inv := range list {
		result[i] = map[string]any{"id": inv.ID, "name": inv.Name}
	}
	return result, nil
}

func (a *App) ListTransfers() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListTransfers(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, t := range list {
		result[i] = map[string]any{
			"id":        t.ID,
			"fromId":    t.FromInventoryID,
			"toId":      t.ToInventoryID,
			"type":      t.Type,
			"createdAt": t.CreatedAt,
		}
	}
	return result, nil
}

// ClientCursorPage is a count-free, keyset-paginated slice of clients. Fetching
// one extra record makes infinite scrolling independent of the table size.
type ClientCursorPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor int64            `json:"nextCursor"`
	HasMore    bool             `json:"hasMore"`
}

func (a *App) ListClientsCursor(afterID int64, limit int, search string) (ClientCursorPage, error) {
	if a.store == nil {
		return ClientCursorPage{}, fmt.Errorf("database not initialized")
	}
	if afterID < 0 {
		afterID = 0
	}
	if limit < 1 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	search = strings.TrimSpace(search)
	if len(search) > 80 {
		search = search[:80]
	}

	queryLimit := int64(limit + 1)
	var rows []db.ListClientsAfterRow
	var err error
	if search == "" {
		unfiltered, queryErr := a.store.ListClientsAfterUnfiltered(a.ctx, db.ListClientsAfterUnfilteredParams{
			AfterID: afterID,
			Limit:   queryLimit,
		})
		err = queryErr
		rows = make([]db.ListClientsAfterRow, len(unfiltered))
		for i, row := range unfiltered {
			rows[i] = db.ListClientsAfterRow(row)
		}
	} else {
		rows, err = a.store.ListClientsAfter(a.ctx, db.ListClientsAfterParams{
			AfterID: afterID,
			Search:  sql.NullString{String: search, Valid: true},
			Limit:   queryLimit,
		})
	}
	if err != nil {
		return ClientCursorPage{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]map[string]any, len(rows))
	for i, row := range rows {
		items[i] = map[string]any{
			"id": row.ID, "name": row.Name, "phone": row.Phone,
			"totalLoyaltyPoints": row.TotalLoyaltyPoints,
			"validLoyaltyPoints": row.ValidLoyaltyPoints,
			"createdAt":          row.CreatedAt,
			"accountBalance":     row.AccountBalance,
		}
	}
	nextCursor := afterID
	if len(rows) > 0 {
		nextCursor = rows[len(rows)-1].ID
	}
	return ClientCursorPage{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// openingFloatCentsForShift converts a specific shift's own opening float
// (frozen at whatever it was when that shift ran, never the current
// settings default) into cents, for crediting the physical-cash row(s) in
// a live or historical per-account balance listing.
func (a *App) openingFloatCentsForShift(shift db.Shift) (usdCents, lbpCents int64, err error) {
	usdCents = shift.OpeningFloatUsd
	if shift.OpeningFloatLbp == 0 {
		return usdCents, 0, nil
	}
	lbpCurrency, err := resolveCurrency(a.ctx, a.store.Queries, "LBP")
	if err != nil {
		return 0, 0, fmt.Errorf("LBP currency: %w", err)
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("settings: %w", err)
	}
	lbpCents, err = roundCentsFromCurrencyUnits(shift.OpeningFloatLbp, lbpCurrency.UnitsPerUsdMicros, settings.RoundingMode)
	if err != nil {
		return 0, 0, fmt.Errorf("opening float LBP: %w", err)
	}
	return usdCents, lbpCents, nil
}

func (a *App) ListShiftBalances(shiftID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.incomeAccountTotals(shiftID)
	if err != nil {
		return nil, err
	}

	// Credit the physical-cash row(s) with this specific shift's own opening
	// float, so the live/expanded balance list matches what closing
	// reconciliation will show (openingFloat + net transactions) instead of
	// looking like a shortage all day. Reading shift.OpeningFloatUsd/Lbp
	// (not the current settings default) keeps this correct for closed
	// shifts too - each shift's float is frozen at whatever it was when
	// that shift ran, never the current default.
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return nil, fmt.Errorf("shift: %w", err)
	}
	usdFloatCents, lbpFloatCents, err := a.openingFloatCentsForShift(shift)
	if err != nil {
		return nil, err
	}

	// Each invoice_payments/client_account_payments/shift_balance_replacements
	// row summed here is already a canonical, exact-cents amount (LBP cash
	// payments were normalized to their rounded-cash-equivalent at checkout
	// time; every other row is validated checkout-exact cents). Summing
	// already-canonical amounts needs no further currency rounding - doing
	// so was redundant at best and, since rounding a sum of parts can land
	// on a different multiple than summing the parts directly, a real
	// source of "balance page total doesn't match invoice total" drift.
	usdFloatApplied := false
	lbpFloatApplied := false
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		onAccount := isNonCollectedIncomeAccount(row.Name)
		onCoupon := strings.EqualFold(strings.TrimSpace(row.Name), "On Coupon")
		balance := databaseInt64(row.Total)
		if !onAccount && isCashAccount(row.Name, row.CurrencyCode) {
			switch {
			case !usdFloatApplied && usdFloatCents != 0 && strings.EqualFold(row.CurrencyCode, "USD"):
				balance += usdFloatCents
				usdFloatApplied = true
			case !lbpFloatApplied && lbpFloatCents != 0 && strings.EqualFold(row.CurrencyCode, "LBP"):
				balance += lbpFloatCents
				lbpFloatApplied = true
			}
		}
		result = append(result, map[string]any{
			"accountId": row.ID, "accountName": row.Name,
			"balance": balance, "currencyCode": row.CurrencyCode,
			"isOnAccount": onAccount, "isOnCoupon": onCoupon,
			"isCollected": !onAccount, "isAutoCounted": onCoupon,
		})
	}
	return result, nil
}

// ReplaceShiftBalance moves part of a shift's recorded balance from one
// cashbox to another without changing the original invoice payments.
// amountCents is always in the accounting currency (USD cents), including
// when the source or destination cashbox is denominated in LBP.
func (a *App) ReplaceShiftBalance(shiftID, fromCashboxID, toCashboxID, amountCents int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	check, err := a.checkShiftBalanceReplacement(shiftID, fromCashboxID, toCashboxID, amountCents)
	if err != nil {
		return err
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		return q.CreateShiftBalanceReplacement(a.ctx, db.CreateShiftBalanceReplacementParams{
			ShiftID: shiftID, FromCashboxID: fromCashboxID, ToCashboxID: toCashboxID, Amount: check.SignedAmount,
		})
	})
}

// ShiftBalanceReplacementPreview is what the balance-swap dialog shows
// before the user confirms - computed with the exact same validation
// ReplaceShiftBalance itself applies, so the preview can never diverge
// from what a submit would actually do.
type ShiftBalanceReplacementPreview struct {
	Valid                    bool   `json:"valid"`
	Reason                   string `json:"reason"`
	SourceRemainingCents     int64  `json:"sourceRemainingCents"`
	DestinationReceivesCents int64  `json:"destinationReceivesCents"`
}

// PreviewShiftBalanceReplacement validates a proposed balance swap without
// applying it. A validation failure is returned as Valid=false with a
// Reason, not a Go error, so the frontend can show it inline.
func (a *App) PreviewShiftBalanceReplacement(shiftID, fromCashboxID, toCashboxID, amountCents int64) (ShiftBalanceReplacementPreview, error) {
	if a.store == nil {
		return ShiftBalanceReplacementPreview{}, fmt.Errorf("database not initialized")
	}
	check, err := a.checkShiftBalanceReplacement(shiftID, fromCashboxID, toCashboxID, amountCents)
	if err != nil {
		return ShiftBalanceReplacementPreview{Reason: err.Error()}, nil
	}
	return ShiftBalanceReplacementPreview{
		Valid:                    true,
		SourceRemainingCents:     check.SourceRemainingCents,
		DestinationReceivesCents: check.DestinationReceivesCents,
	}, nil
}

type shiftBalanceReplacementCheck struct {
	SourceRemainingCents     int64
	DestinationReceivesCents int64
	SignedAmount             int64
}

func (a *App) checkShiftBalanceReplacement(shiftID, fromCashboxID, toCashboxID, amountCents int64) (shiftBalanceReplacementCheck, error) {
	var check shiftBalanceReplacementCheck
	if shiftID <= 0 || fromCashboxID <= 0 || toCashboxID <= 0 || fromCashboxID == toCashboxID {
		return check, fmt.Errorf("choose different balance accounts")
	}
	if amountCents <= 0 {
		return check, fmt.Errorf("enter a positive amount")
	}
	if _, err := a.store.GetShiftByID(a.ctx, shiftID); err != nil {
		return check, fmt.Errorf("shift not found")
	}
	from, err := a.store.GetCashboxAccountByID(a.ctx, fromCashboxID)
	if err != nil {
		return check, fmt.Errorf("source balance account not found")
	}
	to, err := a.store.GetCashboxAccountByID(a.ctx, toCashboxID)
	if err != nil {
		return check, fmt.Errorf("destination balance account not found")
	}
	if isNonCashBalanceAccount(from.Name) || isNonCashBalanceAccount(to.Name) {
		return check, fmt.Errorf("on-account and coupon balances cannot be replaced")
	}
	balances, err := a.incomeAccountTotals(shiftID)
	if err != nil {
		return check, err
	}
	var sourceTotal int64
	for _, balance := range balances {
		if balance.ID == fromCashboxID {
			sourceTotal = databaseInt64(balance.Total)
			break
		}
	}
	sourceMagnitude := absInt64(sourceTotal)
	if amountCents > sourceMagnitude {
		return check, fmt.Errorf("replacement amount exceeds the source balance")
	}
	// A negative balance (for example, a return) must keep its sign when it is
	// moved. The totals query subtracts this value from the source and adds it
	// to the destination, so a signed amount clears the source in either case.
	signedAmount := amountCents
	if sourceTotal < 0 {
		signedAmount = -signedAmount
	}
	check.SourceRemainingCents = sourceMagnitude - amountCents
	check.DestinationReceivesCents = signedAmount
	check.SignedAmount = signedAmount
	return check, nil
}

func isNonCashBalanceAccount(name string) bool {
	name = strings.TrimSpace(name)
	return strings.EqualFold(name, "On Account") || strings.EqualFold(name, "On Coupon")
}

type InvoicePage struct {
	Items []map[string]any `json:"items"`
	Total int64            `json:"total"`
}

type InvoiceFilters struct {
	InvoiceCode string `json:"invoiceCode"`
	Customer    string `json:"customer"`
	Salesperson string `json:"salesperson"`
	PaidTo      string `json:"paidTo"`
	Date        string `json:"date"`
	Total       string `json:"total"`
}

type SalespersonProductSale struct {
	SalespersonID   int64  `json:"salespersonId"`
	SalespersonName string `json:"salespersonName"`
	ProductID       int64  `json:"productId"`
	Barcode         string `json:"barcode"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Color           string `json:"color"`
	Size            string `json:"size"`
	CurrencyCode    string `json:"currencyCode"`
	Quantity        int64  `json:"quantity"`
	Revenue         int64  `json:"revenue"`
}

// ListSalespersonProductSales attributes each sale line to its optional line
// salesperson, falling back to the invoice salesperson when no override was
// selected. Passing the first and last day of a month produces the month-end
// salesperson/product report.
func (a *App) ListSalespersonProductSales(from, to string) ([]SalespersonProductSale, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	fromDate, err := time.Parse("2006-01-02", strings.TrimSpace(from))
	if err != nil {
		return nil, fmt.Errorf("invalid from date %q; use YYYY-MM-DD", from)
	}
	toDate, err := time.Parse("2006-01-02", strings.TrimSpace(to))
	if err != nil {
		return nil, fmt.Errorf("invalid to date %q; use YYYY-MM-DD", to)
	}
	if fromDate.After(toDate) {
		return nil, fmt.Errorf("from date must not be after to date")
	}
	rows, err := a.store.ListSalespersonProductSales(a.ctx, db.ListSalespersonProductSalesParams{
		ReportFrom: fromDate.Format("2006-01-02"),
		ReportTo:   toDate.Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}
	result := make([]SalespersonProductSale, 0, len(rows))
	for _, row := range rows {
		result = append(result, SalespersonProductSale{
			SalespersonID: row.SalespersonID, SalespersonName: row.SalespersonName,
			ProductID: row.ProductID, Barcode: row.Barcode, Code: row.Code, Name: row.Name,
			Color: row.Color, Size: row.Size, CurrencyCode: row.CurrencyCode,
			Quantity: row.Quantity, Revenue: row.Revenue,
		})
	}
	return result, nil
}

func (a *App) ListInvoicesPage(page int, itemsPerPage int) (InvoicePage, error) {
	return a.listInvoicesPage(page, itemsPerPage, InvoiceFilters{})
}

func (a *App) ListInvoicesFilteredPage(page int, itemsPerPage int, filtersJSON string) (InvoicePage, error) {
	filters := InvoiceFilters{}
	if strings.TrimSpace(filtersJSON) != "" {
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return InvoicePage{}, fmt.Errorf("invalid invoice filters: %w", err)
		}
	}
	return a.listInvoicesPage(page, itemsPerPage, filters)
}

func (a *App) listInvoicesPage(page int, itemsPerPage int, filters InvoiceFilters) (InvoicePage, error) {
	if a.store == nil {
		return InvoicePage{}, fmt.Errorf("database not initialized")
	}
	if page < 1 {
		page = 1
	}
	if itemsPerPage < 1 {
		itemsPerPage = 10
	}
	if itemsPerPage > 100 {
		itemsPerPage = 100
	}

	invoices, err := a.store.ListInvoicesPage(a.ctx, db.ListInvoicesPageParams{
		InvoiceFilter:     strings.TrimSpace(filters.InvoiceCode),
		CustomerFilter:    strings.TrimSpace(filters.Customer),
		SalespersonFilter: strings.TrimSpace(filters.Salesperson),
		PaidToFilter:      strings.TrimSpace(filters.PaidTo),
		DateFilter:        strings.TrimSpace(filters.Date),
		TotalFilter:       strings.TrimSpace(filters.Total),
		Limit:             int64(itemsPerPage),
		Offset:            int64(page-1) * int64(itemsPerPage),
	})
	if err != nil {
		return InvoicePage{}, err
	}
	total, err := a.store.CountFilteredInvoices(a.ctx, db.CountFilteredInvoicesParams{
		InvoiceFilter:     strings.TrimSpace(filters.InvoiceCode),
		CustomerFilter:    strings.TrimSpace(filters.Customer),
		SalespersonFilter: strings.TrimSpace(filters.Salesperson),
		PaidToFilter:      strings.TrimSpace(filters.PaidTo),
		DateFilter:        strings.TrimSpace(filters.Date),
		TotalFilter:       strings.TrimSpace(filters.Total),
	})
	if err != nil {
		return InvoicePage{}, err
	}

	// Batch-fetch everything the page needs in a handful of queries instead
	// of a client lookup, a payments lookup, and a cashbox lookup per
	// payment for every row - previously up to ~3x itemsPerPage round trips
	// for one page load.
	clientIDSet := make(map[int64]struct{}, len(invoices))
	invoiceIDs := make([]int64, len(invoices))
	for i, inv := range invoices {
		invoiceIDs[i] = inv.ID
		if inv.ClientID > 0 {
			clientIDSet[inv.ClientID] = struct{}{}
		}
	}
	clientIDs := make([]int64, 0, len(clientIDSet))
	for id := range clientIDSet {
		clientIDs = append(clientIDs, id)
	}
	clientNames := make(map[int64]string, len(clientIDs))
	if len(clientIDs) > 0 {
		clientRows, err := a.store.ListClientNamesByIDs(a.ctx, clientIDs)
		if err != nil {
			return InvoicePage{}, fmt.Errorf("invoice page clients: %w", err)
		}
		for _, row := range clientRows {
			clientNames[row.ID] = row.Name
		}
	}
	accounts, err := a.store.ListCashboxAccounts(a.ctx)
	if err != nil {
		return InvoicePage{}, fmt.Errorf("invoice page cashbox accounts: %w", err)
	}
	accountsByID := make(map[int64]db.CashboxAccount, len(accounts))
	for _, account := range accounts {
		accountsByID[account.ID] = account
	}
	paymentsByInvoice := make(map[int64][]db.InvoicePayment, len(invoices))
	if len(invoiceIDs) > 0 {
		payments, err := a.store.ListInvoicePaymentsByInvoiceIDs(a.ctx, invoiceIDs)
		if err != nil {
			return InvoicePage{}, fmt.Errorf("invoice page payments: %w", err)
		}
		for _, payment := range payments {
			paymentsByInvoice[payment.InvoiceID] = append(paymentsByInvoice[payment.InvoiceID], payment)
		}
	}

	result := make([]map[string]any, len(invoices))
	for i, inv := range invoices {
		clientName := "Walk-in customer"
		if inv.ClientID > 0 {
			if name, ok := clientNames[inv.ClientID]; ok {
				clientName = name
			}
		}
		settlementAccounts := make([]string, 0, len(paymentsByInvoice[inv.ID]))
		for _, payment := range paymentsByInvoice[inv.ID] {
			if payment.Amount <= 0 {
				continue
			}
			if account, ok := accountsByID[payment.CashboxID]; ok {
				settlementAccounts = append(settlementAccounts, settlementAccountLabel(account.Name, account.CurrencyCode))
			}
		}
		if len(settlementAccounts) == 0 {
			if account, ok := accountsByID[inv.CashboxID]; ok {
				settlementAccounts = append(settlementAccounts, settlementAccountLabel(account.Name, account.CurrencyCode))
			}
		}
		result[i] = map[string]any{
			"id": inv.ID, "cashboxId": inv.CashboxID, "invoiceCode": inv.InvoiceCode,
			"clientId": inv.ClientID, "clientName": clientName, "subtotal": inv.Subtotal,
			"discountedTotal": inv.DiscountedTotal, "grandTotal": inv.GrandTotal,
			"createdAt": inv.CreatedAt, "salespersonId": inv.SalespersonID, "salespersonName": inv.SalespersonName,
			"settlementAccounts": settlementAccounts,
		}
	}
	return InvoicePage{Items: result, Total: total}, nil
}

func settlementAccountLabel(name, currencyCode string) string {
	name = strings.TrimSpace(name)
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	if name == "" {
		name = "Cash"
	}
	if currencyCode == "" || strings.Contains(strings.ToUpper(name), currencyCode) {
		return name
	}
	return name + " " + currencyCode
}

func (a *App) GetDailyIncomeByMonth(yearMonth string) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if _, err := time.Parse("2006-01", yearMonth); err != nil {
		return nil, fmt.Errorf("invalid month %q; use YYYY-MM", yearMonth)
	}
	monthStart, _ := time.ParseInLocation("2006-01", yearMonth, time.Local)
	days := monthStart.AddDate(0, 1, -1).Day()
	result := make([]map[string]any, 0, days)
	for day := 1; day <= days; day++ {
		date := fmt.Sprintf("%s-%02d", yearMonth, day)
		invoiceTotal, err := a.store.GetInvoiceIncomeTotalByDate(a.ctx, date)
		if err != nil {
			return nil, err
		}
		total := databaseInt64(invoiceTotal)
		if total != 0 {
			result = append(result, map[string]any{"day": date, "total": float64(total)})
		}
	}
	return result, nil
}

func (a *App) GetTodayIncomeByAccount() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListInvoiceIncomeAccountTotalsByDate(a.ctx, time.Now().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	// See the comment in ListShiftBalances: these are already-canonical
	// exact cents, summed - no re-rounding, to stay consistent with every
	// other income aggregation endpoint.
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		onAccount := isNonCollectedIncomeAccount(r.Name)
		total := databaseInt64(r.Total)
		result[i] = map[string]any{
			"accountId": r.ID, "accountName": r.Name, "currencyCode": r.CurrencyCode, "total": float64(total),
			"isOnAccount": onAccount, "isCollected": !onAccount,
		}
	}
	return result, nil
}

func (a *App) GetCurrentShiftIncomeByAccount() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err == sql.ErrNoRows {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	// Date-scoped, not just shift-scoped: a shift left open across a day
	// boundary (or for days) must not drag prior days' activity - including
	// on-account entries - into what's displayed as "today's" balance.
	rows, err := a.store.ListInvoiceIncomeAccountTotalsByShiftAndDate(a.ctx, db.ListInvoiceIncomeAccountTotalsByShiftAndDateParams{
		InvoiceIncomeShiftID: shift.ID, IncomeReferenceDate: time.Now().Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}
	usdFloatCents, lbpFloatCents, err := a.openingFloatCentsForShift(shift)
	if err != nil {
		return nil, err
	}
	usdFloatApplied := false
	lbpFloatApplied := false
	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		onAccount := isNonCollectedIncomeAccount(row.Name)
		total := databaseInt64(row.Total)
		if !onAccount && isCashAccount(row.Name, row.CurrencyCode) {
			switch {
			case !usdFloatApplied && usdFloatCents != 0 && strings.EqualFold(row.CurrencyCode, "USD"):
				total += usdFloatCents
				usdFloatApplied = true
			case !lbpFloatApplied && lbpFloatCents != 0 && strings.EqualFold(row.CurrencyCode, "LBP"):
				total += lbpFloatCents
				lbpFloatApplied = true
			}
		}
		result[i] = map[string]any{
			"accountId": row.ID, "accountName": row.Name, "currencyCode": row.CurrencyCode, "total": float64(total),
			"isOnAccount": onAccount, "isCollected": !onAccount,
		}
	}
	return result, nil
}

func (a *App) GetCurrentMonth() string {
	now := time.Now()
	return now.Format("2006-01")
}

// GetShiftTotals returns shifts and their totals for the current calendar
// month only - the monthly shift table never displays more than one month
// at a time, so there's no reason to load every shift the shop has ever
// had (which used to grow unbounded as history accumulated).
func (a *App) GetShiftTotals() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.GetShiftTotals(a.ctx, time.Now().Format("2006-01"))
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		total := databaseInt64(r.Total)
		result[i] = map[string]any{
			"id":                  r.ID,
			"isClosed":            r.IsClosed,
			"createdAt":           toLocalTime(r.CreatedAt),
			"closedAt":            toLocalTime(r.ClosedAt),
			"total":               float64(total),
			"priceDeduction":      databaseInt64(r.PriceDeductions),
			"countedUsd":          r.CountedUsd,
			"countedLbp":          r.CountedLbp,
			"countedVisa":         r.CountedVisa,
			"countedWhish":        r.CountedWhish,
			"expectedUsd":         r.ExpectedUsd,
			"expectedLbp":         r.ExpectedLbp,
			"expectedVisa":        r.ExpectedVisa,
			"expectedWhish":       r.ExpectedWhish,
			"closedBySalesperson": r.ClosedBySalespersonName,
		}
	}
	return result, nil
}

// incomeAccountTotals returns each cashbox account's total for a shift,
// with the "On Account" row's total already excluding any on-account
// entry that wasn't recorded today - so a shift left open across a day
// boundary (or for days) doesn't drag prior days' on-account activity into
// what should be "today's" balance.
func (a *App) incomeAccountTotals(shiftID int64) ([]db.ListIncomeAccountTotalsRow, error) {
	rows, err := a.store.ListIncomeAccountTotals(a.ctx, shiftID)
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	stale, err := a.store.StaleOnAccountTotalForShift(a.ctx, db.StaleOnAccountTotalForShiftParams{
		StaleShiftID: shiftID, StaleReferenceDate: today,
	})
	if err != nil {
		return nil, fmt.Errorf("stale on-account total: %w", err)
	}
	staleCents := databaseInt64(stale)
	if staleCents == 0 {
		return rows, nil
	}
	for i := range rows {
		if isNonCollectedIncomeAccount(rows[i].Name) {
			rows[i].Total = databaseInt64(rows[i].Total) - staleCents
		}
	}
	return rows, nil
}

func isNonCollectedIncomeAccount(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "On Account")
}
