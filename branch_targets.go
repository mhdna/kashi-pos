package main

import (
	"fmt"

	db "kashi-pos/db/sqlc"
)

// ListBranchTargetsWithProgress returns this branch's revenue targets
// (pulled down read-only from kashi, see sync_pull.go) each annotated with
// how much has actually been earned in its date range so far -- progress
// is computed here, live, from local invoices, the same as kashi computes
// it centrally from branch_invoices, rather than being synced down as a
// stored number that could go stale between sync ticks.
func (a *App) ListBranchTargetsWithProgress() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	targets, err := a.store.ListBranchTargets(a.ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(targets))
	for i, t := range targets {
		achieved, err := a.store.GetInvoiceIncomeTotalByDateRange(a.ctx, db.GetInvoiceIncomeTotalByDateRangeParams{
			DateFrom: t.DateFrom,
			DateTo:   t.DateTo,
		})
		if err != nil {
			return nil, err
		}
		result[i] = map[string]any{
			"id":                t.ID,
			"dateFrom":          t.DateFrom,
			"dateTo":            t.DateTo,
			"targetAmountCents": t.TargetAmountCents,
			"achievedCents":     databaseInt64(achieved),
			"color":             t.Color,
		}
	}
	return result, nil
}
