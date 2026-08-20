package main

import (
	"database/sql"
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

func (a *App) ListInventoryItems(inventoryID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	items, err := a.store.ListInventoryItems(a.ctx, inventoryID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(items))
	for i, item := range items {
		product, _ := a.store.GetProductByID(a.ctx, item.ProductID)
		name := ""
		if product.Name != "" {
			name = product.Name
		}
		result[i] = map[string]any{
			"productId":   item.ProductID,
			"productName": name,
			"quantity":    item.Quantity,
		}
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

func (a *App) ListClients() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListClients(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, c := range list {
		balanceValue, balanceErr := a.store.GetClientAccountBalance(a.ctx, c.ID)
		if balanceErr != nil {
			return nil, fmt.Errorf("client %s account balance: %w", c.Name, balanceErr)
		}
		result[i] = map[string]any{
			"id":                 c.ID,
			"name":               c.Name,
			"phone":              c.Phone,
			"totalLoyaltyPoints": c.TotalLoyaltyPoints,
			"validLoyaltyPoints": c.ValidLoyaltyPoints,
			"createdAt":          c.CreatedAt,
			"accountBalance":     databaseInt64(balanceValue),
		}
	}
	return result, nil
}

func (a *App) ListClientsPage(search string, page, perPage int64, sortKey, sortOrder string) (map[string]any, error) {
	if a.store == nil || a.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	search = strings.TrimSpace(search)
	if len(search) > 80 {
		search = search[:80]
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	} else if perPage > 100 {
		perPage = 100
	}
	sortColumns := map[string]string{
		"name": "lower(c.name)", "phone": "c.phone",
		"totalLoyaltyPoints": "c.total_loyalty_points",
		"validLoyaltyPoints": "c.valid_loyalty_points",
		"createdAt":          "c.id",
	}
	sortColumn, ok := sortColumns[sortKey]
	if !ok {
		sortColumn = "c.created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	var total int64
	var err error
	if search == "" {
		total, err = a.store.CountClients(a.ctx)
	} else {
		total, err = a.store.CountClientsPage(a.ctx, search)
	}
	if err != nil {
		return nil, err
	}
	query := `WITH selected_clients AS (
		SELECT c.id, c.name, c.phone, c.total_loyalty_points, c.valid_loyalty_points, c.created_at
		FROM clients c
		WHERE trim(?) = '' OR c.name LIKE '%' || ? || '%' COLLATE NOCASE OR c.phone LIKE '%' || ? || '%'
		ORDER BY ` + sortColumn + ` ` + sortOrder + `, c.id DESC LIMIT ? OFFSET ?
	)
	SELECT c.id, c.name, c.phone, c.total_loyalty_points, c.valid_loyalty_points, c.created_at,
		CAST(COALESCE(SUM(cae.amount - COALESCE((
			SELECT SUM(cap.amount) FROM client_account_payments cap WHERE cap.invoice_id = cae.invoice_id
		), 0)), 0) AS INTEGER) AS account_balance
	FROM selected_clients c
	LEFT JOIN client_account_entries cae ON cae.client_id = c.id
	GROUP BY c.id
	ORDER BY ` + sortColumn + ` ` + sortOrder + `, c.id DESC`
	rows, err := a.conn.QueryContext(a.ctx, query, search, search, search, perPage, (page-1)*perPage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, perPage)
	for rows.Next() {
		var id, totalPoints, validPoints, balance int64
		var name, phone, createdAt string
		if err := rows.Scan(&id, &name, &phone, &totalPoints, &validPoints, &createdAt, &balance); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "name": name, "phone": phone, "totalLoyaltyPoints": totalPoints,
			"validLoyaltyPoints": validPoints, "createdAt": createdAt, "accountBalance": balance,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total}, nil
}

func (a *App) ListShifts() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListShifts(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, s := range list {
		result[i] = map[string]any{
			"id":        s.ID,
			"isClosed":  s.IsClosed,
			"createdAt": toLocalTime(s.CreatedAt),
			"closedAt":  toLocalTime(s.ClosedAt),
		}
	}
	return result, nil
}

func (a *App) ListShiftBalances(shiftID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.incomeAccountTotals(shiftID)
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
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		onAccount := isNonCollectedIncomeAccount(row.Name)
		onCoupon := strings.EqualFold(strings.TrimSpace(row.Name), "On Coupon")
		balance := databaseInt64(row.Total)
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

func (a *App) ListInvoices() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListInvoices(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, inv := range list {
		result[i] = map[string]any{
			"id":              inv.ID,
			"cashboxId":       inv.CashboxID,
			"invoiceCode":     inv.InvoiceCode,
			"clientId":        inv.ClientID,
			"subtotal":        inv.Subtotal,
			"discountedTotal": inv.DiscountedTotal,
			"grandTotal":      inv.GrandTotal,
			"createdAt":       inv.CreatedAt,
			"salespersonId":   inv.SalespersonID,
		}
	}
	return result, nil
}

type InvoicePage struct {
	Items []map[string]any `json:"items"`
	Total int64            `json:"total"`
}

func (a *App) ListInvoicesPage(page int, itemsPerPage int) (InvoicePage, error) {
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
		Limit:  int64(itemsPerPage),
		Offset: int64(page-1) * int64(itemsPerPage),
	})
	if err != nil {
		return InvoicePage{}, err
	}
	total, err := a.store.CountInvoices(a.ctx)
	if err != nil {
		return InvoicePage{}, err
	}

	result := make([]map[string]any, len(invoices))
	for i, inv := range invoices {
		clientName := "Walk-in customer"
		if inv.ClientID > 0 {
			if client, clientErr := a.store.GetClientByID(a.ctx, inv.ClientID); clientErr == nil {
				clientName = client.Name
			}
		}
		payments, err := a.store.ListInvoicePayments(a.ctx, inv.ID)
		if err != nil {
			return InvoicePage{}, fmt.Errorf("invoice %s payments: %w", inv.InvoiceCode, err)
		}
		settlementAccounts := make([]string, 0, len(payments))
		for _, payment := range payments {
			if payment.Amount <= 0 {
				continue
			}
			account, err := a.store.GetCashboxAccountByID(a.ctx, payment.CashboxID)
			if err != nil {
				return InvoicePage{}, fmt.Errorf("invoice %s settlement account: %w", inv.InvoiceCode, err)
			}
			settlementAccounts = append(settlementAccounts, settlementAccountLabel(account.Name, account.CurrencyCode))
		}
		if len(settlementAccounts) == 0 {
			if account, accountErr := a.store.GetCashboxAccountByID(a.ctx, inv.CashboxID); accountErr == nil {
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
	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		onAccount := isNonCollectedIncomeAccount(row.Name)
		result[i] = map[string]any{
			"accountId": row.ID, "accountName": row.Name, "currencyCode": row.CurrencyCode, "total": float64(databaseInt64(row.Total)),
			"isOnAccount": onAccount, "isCollected": !onAccount,
		}
	}
	return result, nil
}

func (a *App) GetCurrentMonth() string {
	now := time.Now()
	return now.Format("2006-01")
}

func (a *App) GetShiftTotals() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.GetShiftTotals(a.ctx)
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
