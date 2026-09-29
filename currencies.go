package main

import (
	"context"
	"fmt"
	"regexp"
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

// cashboxAccountColorPattern matches a 6-digit hex color ("#rrggbb"), the
// only format the color picker in Settings ever sends. An empty string is
// also valid and means "no custom color" - the frontend falls back to its
// existing name-based heuristic.
var cashboxAccountColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

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
	// Only active methods are offered. A method kashi no longer has is
	// deactivated rather than deleted (it is the FK target of past payments),
	// so this is what keeps a retired tender off the calculator while its
	// history still resolves.
	//
	// Rows come back in admin-defined sort_order. "On Account"/"On Coupon"
	// are internal bookkeeping placeholders, not selectable tenders, so
	// they're pinned after every real payment method (On Account before On
	// Coupon) regardless of their own sort_order - this keeps them out of the
	// way even on a DB that seeded them before any other method existed.
	accounts, err := a.store.ListActiveCashboxAccounts(a.ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		iSpecial := accounts[i].Name == "On Account" || accounts[i].Name == "On Coupon"
		jSpecial := accounts[j].Name == "On Account" || accounts[j].Name == "On Coupon"
		if iSpecial != jSpecial {
			return !iSpecial
		}
		if iSpecial && jSpecial {
			return accounts[i].Name == "On Account"
		}
		return false
	})
	result := make([]map[string]any, len(accounts))
	for i, acc := range accounts {
		result[i] = map[string]any{
			"id":           acc.ID,
			"name":         acc.Name,
			"displayName":  acc.Name,
			"currencyCode": acc.CurrencyCode,
			"color":        acc.Color,
		}
	}
	return result, nil
}
