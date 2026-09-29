package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestCloseStaleOpenShiftsHonorsSettingAndKeepsTodayOpen(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "shifts.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	stale, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	today, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	// created_at is stored in UTC (CreateShift's default is datetime('now'),
	// which SQLite returns in UTC) -- CloseStaleOpenShifts' own query then
	// applies 'localtime' once to convert it for comparison. Seeding this
	// with an extra 'localtime' shift here double-converts it, which can
	// land back on "today" instead of "yesterday" depending on what wall-
	// clock time the test happens to run at (it did, intermittently). Match
	// the real column convention instead: UTC minus a day.
	if _, err := database.Exec(`UPDATE shifts SET created_at = datetime('now', '-1 day') WHERE id = ?`, stale.ID); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}

	// Explicitly disable first: fresh databases now default this flag to
	// enabled (a secondary hygiene fix, seeded only for new installs) - this
	// test is specifically about the disabled behavior, so don't rely on
	// whatever the default happens to be.
	if err := app.SetAutoCloseStaleShiftsEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := app.closeStaleOpenShifts(); err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.GetShiftByID(ctx, stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.IsClosed != 0 {
		t.Fatal("stale shift closed while automatic closure was disabled")
	}

	if err := app.SetAutoCloseStaleShiftsEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := app.closeStaleOpenShifts(); err != nil {
		t.Fatal(err)
	}
	closed, err := store.GetShiftByID(ctx, stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.IsClosed == 0 || closed.ClosedAt == "" {
		t.Fatalf("stale shift was not closed: %#v", closed)
	}
	stillOpen, err := store.GetShiftByID(ctx, today.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stillOpen.IsClosed != 0 {
		t.Fatal("today's shift was closed")
	}
}
