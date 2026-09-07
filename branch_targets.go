package main

import (
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// ListBranchTargetsWithProgress returns this branch's revenue targets
// (pulled down read-only from kashi, see sync_pull.go) each annotated with
// how much has actually been earned in its date range so far -- progress
// is computed here, live, from local invoices, the same as kashi computes
// it centrally from branch_invoices, rather than being synced down as a
// stored number that could go stale between sync ticks.
//
// Only targets overlapping the current calendar month are returned: the
// till's shift / settings screens are a "how are we doing this month"
// view, not a target archive. Older monthly targets stay in the local
// mirror until kashi drops them; they're just not shown.
func (a *App) ListBranchTargetsWithProgress() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	targets, err := a.store.ListBranchTargetsOverlapping(a.ctx, db.ListBranchTargetsOverlappingParams{
		RangeFrom: firstOfMonth.Format("2006-01-02"),
		RangeTo:   lastOfMonth.Format("2006-01-02"),
	})
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
