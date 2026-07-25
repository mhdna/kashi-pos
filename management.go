package main

import (
	"fmt"
	"time"
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
		result[i] = map[string]any{
			"id":                 c.ID,
			"name":               c.Name,
			"phone":              c.Phone,
			"totalLoyaltyPoints": c.TotalLoyaltyPoints,
			"validLoyaltyPoints": c.ValidLoyaltyPoints,
			"createdAt":          c.CreatedAt,
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
	balances, err := a.store.ListShiftAccountBalances(a.ctx, shiftID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(balances))
	for i, b := range balances {
		account, _ := a.store.GetCashboxAccountByID(a.ctx, b.AccountID)
		name := ""
		if account.Name != "" {
			name = account.Name
		}
		result[i] = map[string]any{
			"accountId":   b.AccountID,
			"accountName": name,
			"balance":     b.Balance,
			"currencyCode": account.CurrencyCode,
		}
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
		result[i] = map[string]any{"accountName": r.Name, "currencyCode": r.CurrencyCode, "total": total}
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
			"id":        r.ID,
			"isClosed":  r.IsClosed,
			"createdAt": toLocalTime(r.CreatedAt),
			"closedAt":  toLocalTime(r.ClosedAt),
			"total":     total,
		}
	}
	return result, nil
}
