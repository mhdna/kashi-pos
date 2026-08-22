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

// validatePaymentMethodInput trims name and resolves+validates the active
// currency shared by CreateCashboxAccount and UpdateCashboxAccount.
func (a *App) validatePaymentMethodInput(name, currencyCode string) (string, db.Currency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", db.Currency{}, fmt.Errorf("payment method name is required")
	}
	if name == "On Account" || name == "On Coupon" {
		return "", db.Currency{}, fmt.Errorf("%q is a reserved payment method name", name)
	}
	currency, err := resolveCurrency(a.ctx, a.store.Queries, currencyCode)
	if err != nil {
		return "", db.Currency{}, fmt.Errorf("currency: %w", err)
	}
	if !currency.IsActive {
		return "", db.Currency{}, fmt.Errorf("currency %q is not active", currency.Code)
	}
	return name, currency, nil
}

// CreateCashboxAccount adds a new payment method (cashbox account) under a
// given currency. Previously the only accounts that could exist were a
// hardcoded seed list created at first run.
func (a *App) CreateCashboxAccount(name, currencyCode string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name, currency, err := a.validatePaymentMethodInput(name, currencyCode)
	if err != nil {
		return nil, err
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

// cashboxAccountColorPattern matches a 6-digit hex color ("#rrggbb"), the
// only format the color picker in Settings ever sends. An empty string is
// also valid and means "no custom color" - the frontend falls back to its
// existing name-based heuristic.
var cashboxAccountColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// UpdateCashboxAccount renames a payment method, changes its currency,
// and/or sets the accent color shown on its calculator button. The two
// internal bookkeeping placeholders ("On Account"/"On Coupon") can't be
// edited - backend logic elsewhere matches them by exact name, so renaming
// one would silently break that.
func (a *App) UpdateCashboxAccount(id int64, name, currencyCode, color string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	existing, err := a.store.GetCashboxAccountByID(a.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("payment method %d: %w", id, err)
	}
	if existing.Name == "On Account" || existing.Name == "On Coupon" {
		return nil, fmt.Errorf("%q is a reserved payment method and cannot be edited", existing.Name)
	}
	name, currency, err := a.validatePaymentMethodInput(name, currencyCode)
	if err != nil {
		return nil, err
	}
	color = strings.TrimSpace(color)
	if color != "" && !cashboxAccountColorPattern.MatchString(color) {
		return nil, fmt.Errorf("color must be a hex value like #2e7d32, got %q", color)
	}
	account, err := a.store.UpdateCashboxAccount(a.ctx, db.UpdateCashboxAccountParams{
		Name:         name,
		CurrencyCode: currency.Code,
		Color:        color,
		ID:           id,
	})
	if err != nil {
		return nil, fmt.Errorf("update payment method: %w", err)
	}
	return map[string]any{
		"id":           account.ID,
		"name":         account.Name,
		"currencyCode": account.CurrencyCode,
		"color":        account.Color,
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
	// ListCashboxAccounts already returns rows in admin-defined sort_order.
	// "On Account"/"On Coupon" are internal bookkeeping placeholders, not
	// selectable tenders, so they're pinned after every real payment method
	// (On Account before On Coupon) regardless of their own sort_order -
	// this keeps them out of the way even on a DB that seeded them before
	// any admin-added methods existed.
	accounts, err := a.store.ListCashboxAccounts(a.ctx)
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

// ReorderCashboxAccounts sets the display order of payment methods from
// Settings. orderedIDs must contain exactly the set of existing "real"
// payment method IDs - every cashbox account except the internal
// "On Account"/"On Coupon" placeholders, which are never shown as a
// selectable tender and so aren't admin-reorderable.
func (a *App) ReorderCashboxAccounts(orderedIDs []int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	accounts, err := a.store.ListCashboxAccounts(a.ctx)
	if err != nil {
		return fmt.Errorf("list payment methods: %w", err)
	}
	reorderable := make(map[int64]bool, len(accounts))
	for _, acc := range accounts {
		if acc.Name == "On Account" || acc.Name == "On Coupon" {
			continue
		}
		reorderable[acc.ID] = true
	}
	if len(orderedIDs) != len(reorderable) {
		return fmt.Errorf("reorder must include exactly the %d existing payment methods, got %d", len(reorderable), len(orderedIDs))
	}
	seen := make(map[int64]bool, len(orderedIDs))
	for _, id := range orderedIDs {
		if !reorderable[id] {
			return fmt.Errorf("payment method %d is not reorderable or does not exist", id)
		}
		if seen[id] {
			return fmt.Errorf("payment method %d listed more than once", id)
		}
		seen[id] = true
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		for i, id := range orderedIDs {
			if err := q.SetCashboxAccountSortOrder(a.ctx, db.SetCashboxAccountSortOrderParams{
				SortOrder: int64(i),
				ID:        id,
			}); err != nil {
				return fmt.Errorf("reorder payment method %d: %w", id, err)
			}
		}
		return nil
	})
}
