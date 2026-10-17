package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// visitorCounterFlag gates the nav-drawer people-visits counter and its
// orange per-day chip in the monthly shift-income table. Turning it off
// only hides that UI -- the visitor_counts rows are left untouched so a
// later re-enable shows the full history again.
const visitorCounterFlag = "visitor_counter"

// localVisitDayKey is the "YYYY-MM-DD" local-calendar-day key visitor
// tallies are bucketed by, matching date(..., 'localtime') used elsewhere.
func localVisitDayKey(t time.Time) string { return t.Format("2006-01-02") }

// GetVisitorCounterEnabled reports whether the people-visits counter is on.
func (a *App) GetVisitorCounterEnabled() bool {
	if a.store == nil {
		return false
	}
	v, err := a.store.GetFeatureFlag(a.ctx, visitorCounterFlag)
	return err == nil && v != 0
}

// SetVisitorCounterEnabled toggles the people-visits counter feature flag.
// Disabling it hides the nav-drawer buttons and the monthly-income chip
// without touching any recorded counts.
func (a *App) SetVisitorCounterEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: visitorCounterFlag, Enabled: boolInt(enabled),
	})
}

// GetVisitorCountToday returns today's people-visits tally, 0 if none has
// been recorded yet.
func (a *App) GetVisitorCountToday() (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	count, err := a.store.GetVisitorCount(a.ctx, localVisitDayKey(time.Now()))
	if err != nil {
		// No row for today yet -- treat as zero rather than an error.
		return 0, nil
	}
	return count, nil
}

// AdjustVisitorCountToday applies delta (the nav-drawer up/down buttons
// pass +1 / -1) to today's tally, clamps it at zero, persists it, and
// returns the new value.
//
// Alongside the tally it appends one visitor_events row per unit the tally
// actually moved (direction "in" for a positive move, "out" for a
// negative one) and, when central sync is configured, queues each of those
// as a sync_outbox entry -- all in one transaction with the tally write,
// so the log can never drift from the number the cashier sees. A press
// that the zero-clamp swallows (▼ at count 0) moves the tally by nothing
// and is deliberately logged as nothing, which keeps
// sum(in) - sum(out) == visitor_counts.count for every day.
func (a *App) AdjustVisitorCountToday(delta int64) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	// Read outside the tx: an unconfigured install must take exactly the
	// same path it does today, and this is a plain settings lookup.
	centralAPIURL, err := a.syncCentralAPIURL()
	if err != nil {
		return 0, err
	}

	now := time.Now()
	day := localVisitDayKey(now)
	occurredAt := now.UTC().Format("2006-01-02 15:04:05")

	var next int64
	queued := false
	txErr := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		current, err := q.GetVisitorCount(a.ctx, day)
		if err != nil {
			current = 0
		}
		next = current + delta
		if next < 0 {
			next = 0
		}
		if err := q.SetVisitorCount(a.ctx, db.SetVisitorCountParams{Day: day, Count: next}); err != nil {
			return err
		}

		// applied is the movement the clamp actually allowed: delta in the
		// common case, 0 when a ▼ hit the floor, and a partial amount only
		// if a caller ever passes |delta| > 1.
		applied := next - current
		direction := "in"
		if applied < 0 {
			direction = "out"
			applied = -applied
		}
		for i := int64(0); i < applied; i++ {
			ref := uuid.NewString()
			if err := q.CreateVisitorEvent(a.ctx, db.CreateVisitorEventParams{
				ClientRef:  ref,
				Day:        day,
				Direction:  direction,
				OccurredAt: occurredAt,
			}); err != nil {
				return err
			}
			if err := queueBranchVisitorEventSync(a.ctx, q, centralAPIURL, branchVisitorEventPayload{
				Ref:        ref,
				Day:        day,
				Direction:  direction,
				OccurredAt: now.UTC(),
			}); err != nil {
				return err
			}
			queued = true
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}

	if queued {
		// Same immediate-flush nudge every other sync-producing write path
		// gives the worker, so a press reaches kashi in ~a tick, not ~15s.
		a.kickSyncWorker()
	}
	return next, nil
}

// GetVisitorCountsForMonth returns a { "YYYY-MM-DD": count } map for every
// day of yearMonth ("2006-01", defaulting to the current month) that has a
// recorded tally.
func (a *App) GetVisitorCountsForMonth(yearMonth string) (map[string]int64, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	yearMonth = strings.TrimSpace(yearMonth)
	if yearMonth == "" {
		yearMonth = a.GetCurrentMonth()
	}
	rows, err := a.store.ListVisitorCountsForMonth(a.ctx, yearMonth+"-%")
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Day] = r.Count
	}
	return out, nil
}
