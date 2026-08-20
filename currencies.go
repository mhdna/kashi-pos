package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	db "kashi-pos/db/sqlc"
)

// resolveCurrency looks up a currency row by code, falling back to the
// default accounting currency when code is empty. Every rounding/conversion
// call site uses this instead of comparing currency codes to a hardcoded
// literal, so adding a new currency in Settings makes it fully usable
// everywhere without a code change.
func resolveCurrency(ctx context.Context, q *db.Queries, code string) (db.Currency, error) {
	if strings.TrimSpace(code) == "" {
		return q.GetDefaultCurrency(ctx)
	}
	currency, err := q.GetCurrencyByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return db.Currency{}, fmt.Errorf("currency %q: %w", code, err)
	}
	return currency, nil
}

// toReceiptCurrency projects a currencies row into the small struct receipt
// printing code needs, keeping receipt_print.go/escpos_*.go free of any
// direct DB dependency.
func toReceiptCurrency(c db.Currency) receiptCurrency {
	return receiptCurrency{
		Code:              c.Code,
		Symbol:            c.Symbol,
		UnitsPerUsdMicros: c.UnitsPerUsdMicros,
		CashRoundingUnit:  c.CashRoundingUnit,
	}
}

// effectiveRoundingCurrencyCode returns the code to display for the
// configured rounding currency, falling back to the default currency if the
// configured one no longer exists (e.g. deactivated after being set) rather
// than failing the whole settings read.
func (a *App) effectiveRoundingCurrencyCode(configuredCode string) string {
	if currency, err := resolveCurrency(a.ctx, a.store.Queries, configuredCode); err == nil {
		return currency.Code
	}
	if def, err := a.store.GetDefaultCurrency(a.ctx); err == nil {
		return def.Code
	}
	return configuredCode
}

func currencyToMap(c db.Currency) map[string]any {
	return map[string]any{
		"code":             c.Code,
		"name":             c.Name,
		"symbol":           c.Symbol,
		"isDefault":        c.IsDefault,
		"isActive":         c.IsActive,
		"unitsPerUsd":      float64(c.UnitsPerUsdMicros) / unitsPerUsdMicroScale,
		"cashRoundingUnit": c.CashRoundingUnit,
	}
}

func (a *App) ListCurrencies() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	currencies, err := a.store.ListCurrencies(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(currencies))
	for i, c := range currencies {
		result[i] = currencyToMap(c)
	}
	return result, nil
}

func (a *App) GetDefaultCurrency() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	c, err := a.store.GetDefaultCurrency(a.ctx)
	if err != nil {
		return nil, err
	}
	return currencyToMap(c), nil
}

// CreateCurrency adds a new currency a shop can use for cash payment
// methods and rounding. unitsPerUsd is how many major units of the new
// currency equal one USD (e.g. 90000 for LBP) - the friendly rate an admin
// actually types in; it's converted once here into the fixed-point
// units_per_usd_micros the rounding math uses. cashRoundingUnit is the
// nearest whole-unit amount physical cash in this currency should round to
// (e.g. 10000 for LBP, 1 for a currency with no physical rounding).
func (a *App) CreateCurrency(code, name, symbol string, unitsPerUsd float64, cashRoundingUnit int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	symbol = strings.TrimSpace(symbol)
	if code == "" || name == "" || symbol == "" {
		return nil, fmt.Errorf("code, name, and symbol are required")
	}
	if unitsPerUsd <= 0 {
		return nil, fmt.Errorf("units per USD must be greater than zero")
	}
	if cashRoundingUnit <= 0 {
		return nil, fmt.Errorf("cash rounding unit must be greater than zero")
	}
	unitsPerUsdMicros := int64(unitsPerUsd*unitsPerUsdMicroScale + 0.5)
	c, err := a.store.CreateCurrency(a.ctx, db.CreateCurrencyParams{
		Code: code, Name: name, Symbol: symbol,
		UnitsPerUsdMicros: unitsPerUsdMicros, CashRoundingUnit: cashRoundingUnit,
	})
	if err != nil {
		return nil, fmt.Errorf("create currency: %w", err)
	}
	return currencyToMap(c), nil
}

// UpdateCurrency edits an existing non-default currency's display and rate.
func (a *App) UpdateCurrency(code, name, symbol string, unitsPerUsd float64, cashRoundingUnit int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	symbol = strings.TrimSpace(symbol)
	if code == "" || name == "" || symbol == "" {
		return nil, fmt.Errorf("code, name, and symbol are required")
	}
	if unitsPerUsd <= 0 {
		return nil, fmt.Errorf("units per USD must be greater than zero")
	}
	if cashRoundingUnit <= 0 {
		return nil, fmt.Errorf("cash rounding unit must be greater than zero")
	}
	unitsPerUsdMicros := int64(unitsPerUsd*unitsPerUsdMicroScale + 0.5)
	c, err := a.store.UpdateCurrency(a.ctx, db.UpdateCurrencyParams{
		Name: name, Symbol: symbol,
		UnitsPerUsdMicros: unitsPerUsdMicros, CashRoundingUnit: cashRoundingUnit,
		Code: code,
	})
	if err != nil {
		return nil, fmt.Errorf("update currency %q: %w", code, err)
	}
	return currencyToMap(c), nil
}

// SetCurrencyActive activates or deactivates a non-default currency.
// Deactivating does not affect existing cashbox accounts or historical
// invoices already denominated in it - it only hides it from future
// selection (new payment methods, rounding-currency choice).
func (a *App) SetCurrencyActive(code string, active bool) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	c, err := a.store.SetCurrencyActive(a.ctx, db.SetCurrencyActiveParams{IsActive: active, Code: code})
	if err != nil {
		return nil, fmt.Errorf("set currency %q active=%v: %w", code, active, err)
	}
	return currencyToMap(c), nil
}

// CreateCashboxAccount adds a new payment method (cashbox account) under a
// given currency. Previously the only accounts that could exist were a
// hardcoded seed list created at first run.
func (a *App) CreateCashboxAccount(name, currencyCode string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("payment method name is required")
	}
	currency, err := resolveCurrency(a.ctx, a.store.Queries, currencyCode)
	if err != nil {
		return nil, fmt.Errorf("currency: %w", err)
	}
	if !currency.IsActive {
		return nil, fmt.Errorf("currency %q is not active", currency.Code)
	}
	account, err := a.store.CreateCashboxAccount(a.ctx, db.CreateCashboxAccountParams{Name: name, CurrencyCode: currency.Code})
	if err != nil {
		return nil, fmt.Errorf("create payment method: %w", err)
	}
	return map[string]any{
		"id":           account.ID,
		"name":         account.Name,
		"currencyCode": account.CurrencyCode,
	}, nil
}

func (a *App) ListPaymentMethods() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	// Older databases may have been created before the special methods were
	// introduced. Ensure their payment cards exist even before a full restart
	// or reseed has occurred.
	for _, special := range []string{"On Account", "On Coupon"} {
		if _, lookupErr := a.store.GetCashboxAccountByName(a.ctx, special); lookupErr != nil {
			if _, createErr := a.store.CreateCashboxAccount(a.ctx, db.CreateCashboxAccountParams{Name: special, CurrencyCode: "USD"}); createErr != nil {
				return nil, fmt.Errorf("create %s payment method: %w", special, createErr)
			}
		}
	}
	accounts, err := a.store.ListCashboxAccounts(a.ctx)
	if err != nil {
		return nil, err
	}
	// Keep the special cards directly after the normal configured methods,
	// with On Account immediately followed by On Coupon.
	sort.SliceStable(accounts, func(i, j int) bool {
		iSpecial := accounts[i].Name == "On Account" || accounts[i].Name == "On Coupon"
		jSpecial := accounts[j].Name == "On Account" || accounts[j].Name == "On Coupon"
		if iSpecial != jSpecial {
			return !iSpecial
		}
		if iSpecial && jSpecial {
			return accounts[i].Name == "On Account"
		}
		return accounts[i].ID < accounts[j].ID
	})
	result := make([]map[string]any, len(accounts))
	for i, acc := range accounts {
		// Keep the database names stable while presenting the shared label
		// used by the first two cash payment cards.
		displayName := acc.Name
		if i < 2 {
			displayName = "Cash"
		}
		result[i] = map[string]any{
			"id":           acc.ID,
			"name":         acc.Name,
			"displayName":  displayName,
			"currencyCode": acc.CurrencyCode,
		}
	}
	return result, nil
}
