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
	rows, err := a.store.ListShiftPaymentTotals(a.ctx, shiftID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		onAccount := strings.EqualFold(strings.TrimSpace(row.AccountName), "On Account")
		result = append(result, map[string]any{
			"accountId": row.AccountID, "accountName": row.AccountName,
			"balance": databaseInt64(row.Total), "currencyCode": row.CurrencyCode,
			"isOnAccount": onAccount, "isCollected": !onAccount,
		})
	}
	return result, nil
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
		itemsPerPage = 20
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
	rows, err := a.store.GetDailyIncomeByMonth(a.ctx, yearMonth)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		day := ""
		if d, ok := r.Day.(string); ok {
			day = d
		}
		total := float64(0)
		if r.Total.Valid {
			total = r.Total.Float64
		}
		result[i] = map[string]any{"day": day, "total": total}
	}
	return result, nil
}

func (a *App) GetTodayIncomeByAccount() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.GetTodayIncomeByAccount(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		total := float64(0)
		if r.Total.Valid {
			total = r.Total.Float64
		}
		onAccount := strings.EqualFold(strings.TrimSpace(r.Name), "On Account")
		result[i] = map[string]any{
			"accountName": r.Name, "currencyCode": r.CurrencyCode, "total": total,
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
		total := float64(0)
		if t, ok := r.Total.(int64); ok {
			total = float64(t)
		}
		result[i] = map[string]any{
			"id":             r.ID,
			"isClosed":       r.IsClosed,
			"createdAt":      toLocalTime(r.CreatedAt),
			"closedAt":       toLocalTime(r.ClosedAt),
			"total":          total,
			"priceDeduction": databaseInt64(r.PriceDeductions),
		}
	}
	return result, nil
}
