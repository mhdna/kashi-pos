package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Checkout reads (pricing, payments, coupons) before its first write, while
// the sync worker keeps committing small writes on other pooled connections.
// A write transaction shaped like that must wait its turn for the write lock,
// never fail with "database is locked" (5) or (517).
func TestDatabaseDSN_ReadThenWriteTxWaitsForConcurrentWriter(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", databaseDSN(filepath.Join(t.TempDir(), "lock.db")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.ExecContext(ctx, `CREATE TABLE counters(id INTEGER PRIMARY KEY, n INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO counters(id, n) VALUES (1, 0), (2, 0)`); err != nil {
		t.Fatal(err)
	}

	// Stands in for the sync worker: a stream of small autocommit writes.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = conn.ExecContext(ctx, `UPDATE counters SET n = n + 1 WHERE id = 2`)
			time.Sleep(time.Millisecond)
		}
	}()
	defer func() { close(stop); <-done }()

	const sales = 100
	for i := 0; i < sales; i++ {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("sale %d: begin: %v", i, err)
		}
		var n int64
		if err := tx.QueryRowContext(ctx, `SELECT n FROM counters WHERE id = 1`).Scan(&n); err != nil {
			_ = tx.Rollback()
			t.Fatalf("sale %d: read: %v", i, err)
		}
		// Give the background writer room to commit between the read and
		// the write, as the sync worker does mid-checkout.
		time.Sleep(time.Millisecond)
		if _, err := tx.ExecContext(ctx, `UPDATE counters SET n = ? WHERE id = 1`, n+1); err != nil {
			_ = tx.Rollback()
			t.Fatalf("sale %d: write: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("sale %d: commit: %v", i, err)
		}
	}

	var n int64
	if err := conn.QueryRowContext(ctx, `SELECT n FROM counters WHERE id = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != sales {
		t.Fatalf("counter = %d; want %d", n, sales)
	}
}
