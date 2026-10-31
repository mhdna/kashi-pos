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

func TestModifierWatchLetsGoOfAKeyNothingOnTheTillHolds(t *testing.T) {
	var w modifierWatch
	start := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	// AnyDesk pressed Win and never let go.
	for s := 0; s < 3; s++ {
		if w.check(modWin, true, start.Add(time.Duration(s)*time.Second)) {
			t.Fatalf("let go after %ds, want %v", s, stuckModifierAfter)
		}
	}
	if !w.check(modWin, true, start.Add(3*time.Second)) {
		t.Fatal("Win held by nothing on the till for 3s wasn't let go")
	}
	// Let go: the next press starts the clock again.
	w.check(modWin, false, start.Add(4*time.Second))
	if w.check(modWin, true, start.Add(5*time.Second)) {
		t.Fatal("a new press was let go at once")
	}
}

func TestModifierWatchLeavesAKeyHeldOnTheTill(t *testing.T) {
	var w modifierWatch
	start := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	w.pressed(modShift, true)
	for s := 0; s < 10; s++ {
		if w.check(modShift, true, start.Add(time.Duration(s)*time.Second)) {
			t.Fatal("let go of Shift held on the till's keyboard")
		}
	}
	// Released on the keyboard but Windows still holds it: stuck.
	w.pressed(modShift, false)
	w.check(modShift, true, start.Add(10*time.Second))
	if !w.check(modShift, true, start.Add(13*time.Second)) {
		t.Fatal("Shift held by nothing on the till for 3s wasn't let go")
	}
}

func TestModifierWatchForgetsAMissedKeyUp(t *testing.T) {
	var w modifierWatch
	start := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	w.pressed(modCtrl, true)
	// The key-up never reached the listener, but Windows says Ctrl is up.
	w.check(modCtrl, false, start)
	w.check(modCtrl, true, start.Add(time.Second))
	if !w.check(modCtrl, true, start.Add(4*time.Second)) {
		t.Fatal("a missed key-up kept Ctrl from ever being let go")
	}
}

// Letting go of stuck keys is on for a fresh till, and the Settings page's
// switch turns it off and on for the device listener too.
func TestReleaseStuckKeysOnByDefault(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "keys.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}
	t.Cleanup(func() { stuckKeysLeftAlone.Store(false) })

	if !app.releaseStuckKeys() {
		t.Fatal("letting go of stuck keys must be on until turned off")
	}
	for _, want := range []bool{false, true} {
		if err := app.SetReleaseStuckKeys(want); err != nil {
			t.Fatal(err)
		}
		if got := app.releaseStuckKeys(); got != want {
			t.Fatalf("after SetReleaseStuckKeys(%v), releaseStuckKeys() = %v", want, got)
		}
		if stuckKeysLeftAlone.Load() == want {
			t.Fatalf("after SetReleaseStuckKeys(%v), the device listener wasn't told", want)
		}
	}
}
