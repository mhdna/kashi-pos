package main

import (
	"fmt"
	"strings"
	"time"

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
func (a *App) AdjustVisitorCountToday(delta int64) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	day := localVisitDayKey(time.Now())
	current, err := a.store.GetVisitorCount(a.ctx, day)
	if err != nil {
		current = 0
	}
	next := current + delta
	if next < 0 {
		next = 0
	}
	if err := a.store.SetVisitorCount(a.ctx, db.SetVisitorCountParams{Day: day, Count: next}); err != nil {
		return 0, err
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
