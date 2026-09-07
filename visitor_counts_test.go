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
	// Match the production DSN's locking behavior (app.go): WAL + a busy
	// timeout so a background sync goroutine holding a write lock makes a
	// concurrent write wait, not fail with SQLITE_BUSY.
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "visitors.db")+
		"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
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

// countVisitorEvents returns how many visitor_events rows match direction
// (pass "" for all), and how many sync_outbox rows have entity_type
// 'visitor_event'.
func countVisitorEvents(t *testing.T, app *App, direction string) (events, outbox int) {
	t.Helper()
	q := `SELECT COUNT(*) FROM visitor_events`
	args := []any{}
	if direction != "" {
		q += ` WHERE direction = ?`
		args = append(args, direction)
	}
	if err := app.conn.QueryRowContext(app.ctx, q, args...).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := app.conn.QueryRowContext(app.ctx,
		`SELECT COUNT(*) FROM sync_outbox WHERE entity_type = 'visitor_event'`).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	return events, outbox
}

func TestAdjustVisitorCountToday_LogsEventsAndQueuesSync(t *testing.T) {
	app := newVisitorTestApp(t)
	// Central sync configured: every applied press should also land in the
	// outbox.
	if _, err := app.conn.ExecContext(app.ctx,
		`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id, central_api_url)
		 VALUES ('Test','','','80mm','','',80,297,'system',0,'half_up',0,0,'http://central.example')`); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := app.AdjustVisitorCountToday(1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.AdjustVisitorCountToday(-1); err != nil {
		t.Fatal(err)
	}
	// count is 2 here. A -5 press applies only the 2 units that fit above
	// the floor -- two real departures get logged, the 3 beyond zero are
	// swallowed.
	if _, err := app.AdjustVisitorCountToday(-5); err != nil {
		t.Fatal(err)
	}
	// count is 0 here: this press moves nothing and must log nothing.
	if _, err := app.AdjustVisitorCountToday(-1); err != nil {
		t.Fatal(err)
	}

	ins, _ := countVisitorEvents(t, app, "in")
	outs, _ := countVisitorEvents(t, app, "out")
	total, outbox := countVisitorEvents(t, app, "")
	if ins != 3 || outs != 3 || total != 6 {
		t.Fatalf("events in=%d out=%d total=%d; want 3/3/6", ins, outs, total)
	}
	if outbox != 6 {
		t.Fatalf("visitor_event outbox rows = %d; want 6", outbox)
	}

	// The clamp guarantee: sum(in) - sum(out) == the day's tally.
	if got, _ := app.GetVisitorCountToday(); got != int64(ins-outs) {
		t.Fatalf("tally %d != in-out %d", got, ins-outs)
	}
}

func TestAdjustVisitorCountToday_NoSyncWhenCentralUnset(t *testing.T) {
	app := newVisitorTestApp(t)

	if _, err := app.AdjustVisitorCountToday(1); err != nil {
		t.Fatal(err)
	}
	total, outbox := countVisitorEvents(t, app, "")
	if total != 1 {
		t.Fatalf("events = %d; want 1 (log is unconditional)", total)
	}
	if outbox != 0 {
		t.Fatalf("outbox rows = %d; want 0 (sync not configured)", outbox)
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
