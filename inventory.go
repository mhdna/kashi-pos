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

// ListInventoryCursor feeds the Inventory page's scrolling list: the same
// products, search and cursor as the product search dialog
// (ListProductsCursor), with each product's stock at the other branches
// attached as "branches" ({branchCode, quantity, priceCents}) and their sum
// as "otherStock". This till's own stock isn't known locally.
func (a *App) ListInventoryCursor(afterID int64, limit int, search string) (ProductCursorPage, error) {
	page, err := a.ListProductsCursor(afterID, limit, search)
	if err != nil || len(page.Items) == 0 {
		return page, err
	}
	ids := make([]int64, len(page.Items))
	for i, item := range page.Items {
		ids[i], _ = item["id"].(int64)
	}
	rows, err := a.store.ListBranchStockForProducts(a.ctx, ids)
	if err != nil {
		return ProductCursorPage{}, fmt.Errorf("load branch stock: %w", err)
	}
	byProduct := map[int64][]map[string]any{}
	totals := map[int64]int64{}
	for _, r := range rows {
		byProduct[r.ProductID] = append(byProduct[r.ProductID], map[string]any{
			"branchCode": r.BranchCode,
			"quantity":   r.Quantity,
			"priceCents": r.PriceCents,
		})
		totals[r.ProductID] += r.Quantity
	}
	for i, item := range page.Items {
		branches := byProduct[ids[i]]
		if branches == nil {
			branches = []map[string]any{}
		}
		item["branches"] = branches
		item["otherStock"] = totals[ids[i]]
	}
	return page, nil
}
