package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// What checkout does when a line needs more stock than the branch has
// (settings.oversell_mode, set by kashi per branch):
const (
	oversellAllow = "allow" // sell; kashi flags the negative stock
	oversellWarn  = "warn"  // the cashier confirms first (see CheckOversell)
	oversellBlock = "block" // refuse the sale
)

// OversellLine is one cart product the branch doesn't have enough of.
type OversellLine struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Requested int64  `json:"requested"`
	Available int64  `json:"available"`
}

// OversellCheck is CheckOversell's answer: the branch's mode and the cart
// lines short of stock (empty when everything is covered or stock isn't
// known on this till).
type OversellCheck struct {
	Mode  string         `json:"mode"`
	Lines []OversellLine `json:"lines"`
}

func (a *App) SetOversellMode(mode string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	switch mode {
	case oversellAllow, oversellWarn, oversellBlock:
	default:
		return fmt.Errorf("invalid oversell mode %q", mode)
	}
	return a.store.SetOversellMode(a.ctx, mode)
}

// homeBranchCode is this till's branch in the branch_stock mirror. Until
// kashi has told it (see pushSettings), it's the till's own code -- which is
// the branch code for a branch's first (or only) register.
func homeBranchCode(settings db.Setting) string {
	if code := strings.TrimSpace(settings.HomeBranchCode); code != "" {
		return code
	}
	return strings.TrimSpace(settings.BranchCode)
}

// CheckOversell is called by the sale screen before checkout so a "warn"
// branch can ask the cashier to confirm. It prices the cart the same way
// checkout does, so it sees exactly the products checkout will sell.
func (a *App) CheckOversell(itemsJSON string) (OversellCheck, error) {
	check := OversellCheck{Mode: oversellAllow, Lines: []OversellLine{}}
	if a.store == nil {
		return check, fmt.Errorf("database not initialized")
	}
	items, err := parseSalesItems(itemsJSON, a.unknownBarcode())
	if err != nil {
		return check, err
	}
	pricing, err := a.loadPricingContext()
	if err != nil {
		return check, err
	}
	check.Mode = pricing.Settings.OversellMode
	if check.Mode == oversellAllow {
		return check, nil
	}
	totals, err := a.priceInvoiceTotals(a.store.Queries, pricing.InventoryID, items, pricing.Settings, pricing.RoundingMode, pricing.CustomPricingEnabled, pricing.PerUnitPricingEnabled)
	if err != nil {
		return check, err
	}
	check.Lines, err = oversoldLines(a.ctx, a.store.Queries, pricing.Settings, totals.Products)
	return check, err
}

// oversoldLines compares each product's cart quantity with what the branch
// has: kashi's on-hand for this branch (the branch_stock mirror, which
// already reflects every register's synced sales) minus this till's own
// sales kashi hadn't received when that figure was pulled. A product kashi
// reports no stock row for counts as 0. When the mirror holds nothing for
// this branch at all (sync off, or never pulled) stock is unknown and
// nothing is reported -- a till must never refuse sales for lack of data.
func oversoldLines(ctx context.Context, q *db.Queries, settings db.Setting, products []pricedInvoiceProduct) ([]OversellLine, error) {
	lines := []OversellLine{}
	branchCode := homeBranchCode(settings)
	if branchCode == "" {
		return lines, nil
	}
	rows, err := q.CountBranchStockRows(ctx, branchCode)
	if err != nil {
		return nil, fmt.Errorf("branch stock: %w", err)
	}
	if rows == 0 {
		return lines, nil
	}

	type need struct {
		product db.Product
		qty     int64
	}
	needs := []*need{}
	byID := map[int64]*need{}
	for _, p := range products {
		if n, ok := byID[p.product.ID]; ok {
			n.qty += p.qty
			continue
		}
		n := &need{product: p.product, qty: p.qty}
		byID[p.product.ID] = n
		needs = append(needs, n)
	}

	for _, n := range needs {
		if !n.product.Uid.Valid || n.product.Uid.String == "" {
			continue // a till-only product kashi doesn't track
		}
		var onHand int64
		asOf := sql.NullString{}
		stock, err := q.GetBranchStockRow(ctx, db.GetBranchStockRowParams{BranchCode: branchCode, VariantUid: n.product.Uid.String})
		switch {
		case err == nil:
			onHand = stock.Quantity
			asOf = sql.NullString{String: stock.UpdatedAt, Valid: true}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return nil, fmt.Errorf("branch stock %s: %w", n.product.Code, err)
		}
		pending, err := q.SumProductSoldNotOnCentral(ctx, db.SumProductSoldNotOnCentralParams{ProductID: n.product.ID, AsOf: asOf})
		if err != nil {
			return nil, fmt.Errorf("unsynced sales %s: %w", n.product.Code, err)
		}
		available := onHand - databaseInt64(pending)
		if n.qty > available {
			lines = append(lines, OversellLine{Code: n.product.Code, Name: n.product.Name, Requested: n.qty, Available: available})
		}
	}
	return lines, nil
}

// oversellError is checkout's refusal in "block" mode.
func oversellError(lines []OversellLine) error {
	parts := make([]string, 0, len(lines))
	for _, l := range lines {
		parts = append(parts, fmt.Sprintf("%s (%d in stock, %d requested)", l.Code, max(l.Available, 0), l.Requested))
	}
	return fmt.Errorf("not enough stock: %s", strings.Join(parts, ", "))
}
