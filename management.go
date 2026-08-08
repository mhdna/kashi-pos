package main

import (
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
	rows, err := a.incomeAccountTotals("", "", shiftID)
	if err != nil {
		return nil, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		onAccount := isNonCollectedIncomeAccount(row.Name)
		onCoupon := strings.EqualFold(strings.TrimSpace(row.Name), "On Coupon")
		balance, roundErr := roundCentsByCurrency(databaseInt64(row.Total), settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
		if roundErr != nil {
			return nil, roundErr
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
	if shiftID <= 0 || fromCashboxID <= 0 || toCashboxID <= 0 || fromCashboxID == toCashboxID {
		return fmt.Errorf("choose different balance accounts")
	}
	if amountCents <= 0 {
		return fmt.Errorf("enter a positive amount")
	}
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return fmt.Errorf("shift not found")
	}
	from, err := a.store.GetCashboxAccountByID(a.ctx, fromCashboxID)
	if err != nil {
		return fmt.Errorf("source balance account not found")
	}
	to, err := a.store.GetCashboxAccountByID(a.ctx, toCashboxID)
	if err != nil {
		return fmt.Errorf("destination balance account not found")
	}
	if isNonCashBalanceAccount(from.Name) || isNonCashBalanceAccount(to.Name) {
		return fmt.Errorf("on-account and coupon balances cannot be replaced")
	}
	balances, err := a.incomeAccountTotals("", "", shiftID)
	if err != nil {
		return err
	}
	var sourceTotal int64
	for _, balance := range balances {
		if balance.ID == fromCashboxID {
			sourceTotal = databaseInt64(balance.Total)
			break
		}
	}
	sourceMagnitude := sourceTotal
	if sourceMagnitude < 0 {
		sourceMagnitude = -sourceMagnitude
	}
	if amountCents > sourceMagnitude {
		return fmt.Errorf("replacement amount exceeds the source balance")
	}
	// A negative balance (for example, a return) must keep its sign when it is
	// moved. The totals query subtracts this value from the source and adds it
	// to the destination, so a signed amount clears the source in either case.
	signedAmount := amountCents
	if sourceTotal < 0 {
		signedAmount = -signedAmount
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		return q.CreateShiftBalanceReplacement(a.ctx, db.CreateShiftBalanceReplacementParams{
			ShiftID: shift.ID, FromCashboxID: fromCashboxID, ToCashboxID: toCashboxID, Amount: signedAmount,
		})
	})
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
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	days := monthStart.AddDate(0, 1, -1).Day()
	result := make([]map[string]any, 0, days)
	for day := 1; day <= days; day++ {
		date := fmt.Sprintf("%s-%02d", yearMonth, day)
		rows, err := a.incomeAccountTotals(date, "", 0)
		if err != nil {
			return nil, err
		}
		var total int64
		for _, row := range rows {
			if !isNonCollectedIncomeAccount(row.Name) {
				total += databaseInt64(row.Total)
			}
		}
		if total != 0 {
			total, err = roundCentsByCurrency(total, settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
			if err != nil {
				return nil, err
			}
			result = append(result, map[string]any{"day": date, "total": float64(total)})
		}
	}
	return result, nil
}

func (a *App) GetTodayIncomeByAccount() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.incomeAccountTotals(time.Now().Format("2006-01-02"), "", 0)
	if err != nil {
		return nil, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		onAccount := isNonCollectedIncomeAccount(r.Name)
		total, roundErr := roundCentsByCurrency(databaseInt64(r.Total), settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
		if roundErr != nil {
			return nil, roundErr
		}
		result[i] = map[string]any{
			"accountId": r.ID, "accountName": r.Name, "currencyCode": r.CurrencyCode, "total": float64(total),
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
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		balances, balanceErr := a.incomeAccountTotals("", "", r.ID)
		if balanceErr != nil {
			return nil, balanceErr
		}
		var total int64
		for _, balance := range balances {
			if !isNonCollectedIncomeAccount(balance.Name) {
				total += databaseInt64(balance.Total)
			}
		}
		total, roundErr := roundCentsByCurrency(total, settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
		if roundErr != nil {
			return nil, roundErr
		}
		result[i] = map[string]any{
			"id":             r.ID,
			"isClosed":       r.IsClosed,
			"createdAt":      toLocalTime(r.CreatedAt),
			"closedAt":       toLocalTime(r.ClosedAt),
			"total":          float64(total),
			"priceDeduction": databaseInt64(r.PriceDeductions),
		}
	}
	return result, nil
}

func (a *App) incomeAccountTotals(date, month string, shiftID int64) ([]db.ListIncomeAccountTotalsRow, error) {
	return a.store.ListIncomeAccountTotals(a.ctx, db.ListIncomeAccountTotalsParams{
		IncomeDate: date, IncomeMonth: month, IncomeShiftID: shiftID,
	})
}

func isNonCollectedIncomeAccount(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "On Account")
}
