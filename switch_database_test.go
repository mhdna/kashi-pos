package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// Switching to a database that won't open leaves the workers stopped; quitting
// afterwards must not close their stop channels a second time.
func TestShutdownAfterFailedDatabaseSwitch(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "first.db"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, conn: conn, store: db.NewStore(conn)}
	// As startSyncWorker/startWSClient leave them.
	app.syncWorkerStop = make(chan struct{})
	app.wsClientStop = make(chan struct{})

	if err := app.switchDatabase(filepath.Join(t.TempDir(), "missing.db"), false); err == nil {
		t.Fatal("switching to a missing database should fail")
	}
	app.shutdown(ctx)
}
