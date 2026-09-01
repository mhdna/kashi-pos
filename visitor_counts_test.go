package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func newVisitorTestApp(t *testing.T) *App {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "visitors.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: ctx, store: db.NewStore(database), conn: database}
}

func TestAdjustVisitorCountToday_IncrementsClampsAndPersists(t *testing.T) {
	app := newVisitorTestApp(t)

	if got, err := app.GetVisitorCountToday(); err != nil || got != 0 {
		t.Fatalf("initial count = %d, %v; want 0, nil", got, err)
	}

	for i, want := range []int64{1, 2, 3} {
		got, err := app.AdjustVisitorCountToday(1)
		if err != nil || got != want {
			t.Fatalf("increment %d = %d, %v; want %d, nil", i, got, err, want)
		}
	}

	// Clamps at zero rather than going negative.
	if got, _ := app.AdjustVisitorCountToday(-10); got != 0 {
		t.Fatalf("clamped count = %d; want 0", got)
	}

	if got, _ := app.GetVisitorCountToday(); got != 0 {
		t.Fatalf("persisted count = %d; want 0", got)
	}
}

func TestGetVisitorCountsForMonth(t *testing.T) {
	app := newVisitorTestApp(t)

	now := time.Now()
	thisMonth := now.Format("2006-01")
	today := now.Format("2006-01-02")
	otherMonthDay := now.AddDate(0, -1, 0).Format("2006-01-02")

	if _, err := app.AdjustVisitorCountToday(5); err != nil {
		t.Fatal(err)
	}
	if err := app.store.SetVisitorCount(app.ctx, db.SetVisitorCountParams{Day: otherMonthDay, Count: 9}); err != nil {
		t.Fatal(err)
	}

	counts, err := app.GetVisitorCountsForMonth(thisMonth)
	if err != nil {
		t.Fatal(err)
	}
	if counts[today] != 5 {
		t.Fatalf("counts[%s] = %d; want 5", today, counts[today])
	}
	if _, ok := counts[otherMonthDay]; ok {
		t.Fatalf("counts leaked a day outside the requested month: %v", counts)
	}
}

func TestVisitorCounterFlagToggle(t *testing.T) {
	app := newVisitorTestApp(t)

	if app.GetVisitorCounterEnabled() {
		t.Fatal("counter should be off by default")
	}
	if err := app.SetVisitorCounterEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !app.GetVisitorCounterEnabled() {
		t.Fatal("counter should be on after enabling")
	}

	// Disabling the flag must not wipe recorded counts.
	if _, err := app.AdjustVisitorCountToday(4); err != nil {
		t.Fatal(err)
	}
	if err := app.SetVisitorCounterEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.GetVisitorCountToday(); got != 4 {
		t.Fatalf("count after disabling flag = %d; want 4 (data must be retained)", got)
	}
}
