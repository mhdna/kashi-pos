package main

import (
	"fmt"
)

// ListBranchStock feeds the Inventory page's "Other Branches" search --
// every other branch's stock this till has pulled down (see
// applySyncedBranchStock/sync_pull_coupons.go), filtered locally by
// product name/code/barcode. The local mirror is small enough that a
// direct per-keystroke query (debounced on the frontend) is simpler than
// building server-side pagination for it.
func (a *App) ListBranchStock(search string) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.SearchBranchStock(a.ctx, search)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, r := range rows {
		result[i] = map[string]any{
			"branchCode":  r.BranchCode,
			"barcode":     r.Barcode,
			"quantity":    r.Quantity,
			"priceCents":  r.PriceCents,
			"productName": r.ProductName,
			"productCode": r.ProductCode,
		}
	}
	return result, nil
}
