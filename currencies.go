package main

import "fmt"

func (a *App) ListCurrencies() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	currencies, err := a.queries.ListCurrencies(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(currencies))
	for i, c := range currencies {
		result[i] = map[string]any{
			"code":                   c.Code,
			"name":                   c.Name,
			"symbol":                 c.Symbol,
			"isDefault":              c.IsDefault,
			"isActive":               c.IsActive,
			"valueInDefaultCurrency": c.ValueInDefaultCurrency,
		}
	}
	return result, nil
}

func (a *App) GetDefaultCurrency() (map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	c, err := a.queries.GetDefaultCurrency(a.ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"code":                   c.Code,
		"name":                   c.Name,
		"symbol":                 c.Symbol,
		"isDefault":              c.IsDefault,
		"valueInDefaultCurrency": c.ValueInDefaultCurrency,
	}, nil
}

func (a *App) ListPaymentMethods() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	accounts, err := a.queries.ListCashboxAccounts(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(accounts))
	for i, acc := range accounts {
		result[i] = map[string]any{
			"id":           acc.ID,
			"name":         acc.Name,
			"currencyCode": acc.CurrencyCode,
		}
	}
	return result, nil
}
