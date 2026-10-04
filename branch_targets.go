package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// ListBranchTargetsWithProgress returns this branch's revenue targets
// (pulled down read-only from kashi, see sync_pull.go) each annotated with
// how much the branch has earned in its date range so far.
//
// A branch can have several tills sharing one target, so progress is
// kashi's branch-wide figure (every register's synced sales, refreshed each
// sync tick by pullTargetsProgress) plus this till's own sales kashi hadn't
// received when that figure was fetched -- so a sale moves the bar
// immediately, offline included, and is never counted twice once it syncs.
// Without a figure from kashi yet (sync off, or an older kashi), it falls
// back to this till's own sales.
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
		achieved, err := a.targetAchievedCents(t)
		if err != nil {
			return nil, err
		}
		result[i] = map[string]any{
			"id":                t.ID,
			"dateFrom":          t.DateFrom,
			"dateTo":            t.DateTo,
			"targetAmountCents": t.TargetAmountCents,
			"achievedCents":     achieved,
			"color":             t.Color,
		}
	}
	return result, nil
}

func (a *App) targetAchievedCents(t db.BranchTarget) (int64, error) {
	// kashi reports progress by its own target id, mirrored here as remote_id.
	central, err := a.store.GetBranchTargetProgress(a.ctx, t.RemoteID)
	if errors.Is(err, sql.ErrNoRows) {
		local, err := a.store.GetInvoiceIncomeTotalByDateRange(a.ctx, db.GetInvoiceIncomeTotalByDateRangeParams{
			DateFrom: t.DateFrom,
			DateTo:   t.DateTo,
		})
		if err != nil {
			return 0, err
		}
		return databaseInt64(local), nil
	}
	if err != nil {
		return 0, err
	}
	pending, err := a.store.SumInvoiceTotalNotOnCentralByDateRange(a.ctx, db.SumInvoiceTotalNotOnCentralByDateRangeParams{
		DateFrom: t.DateFrom,
		DateTo:   t.DateTo,
		AsOf:     sql.NullString{String: central.FetchedAt, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	return central.AchievedCents + databaseInt64(pending), nil
}

// pullTargetsProgress replaces the stored branch-wide progress with kashi's
// current figures. The stored set is replaced whole, so a target kashi
// stopped reporting falls back to local-only progress.
func (a *App) pullTargetsProgress(ctx context.Context, client *centralClient) error {
	progress, ok, err := client.GetTargetsProgress(ctx)
	if err != nil || !ok {
		return err
	}
	fetchedAt := time.Now().UTC().Format(sqliteTimeLayout)
	return a.store.ExecTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteBranchTargetProgress(ctx); err != nil {
			return err
		}
		for _, p := range progress {
			if err := q.InsertBranchTargetProgress(ctx, db.InsertBranchTargetProgressParams{
				TargetID:      p.ID,
				AchievedCents: p.Achieved,
				FetchedAt:     fetchedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
