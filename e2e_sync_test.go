//go:build e2e

package main

// Temporary end-to-end harness: runs the real sync code against a copy of a
// real till database and a locally running kashi. Not part of the normal
// test suite (build tag e2e). Usage:
//
//	E2E_DB=/path/copy.db E2E_URL=http://localhost:8090 E2E_CODE=E2E01 E2E_KEY=... \
//	  go test -tags e2e -run TestE2E -v -timeout 30m .

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func e2eApp(t *testing.T) (*App, *db.Store, *sql.DB) {
	t.Helper()
	path := os.Getenv("E2E_DB")
	if path == "" {
		t.Skip("E2E_DB not set")
	}
	ctx := context.Background()
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := applyMigrations(ctx, conn, path, true); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE settings SET central_api_url = ?, branch_code = ?, api_key = ?`,
		os.Getenv("E2E_URL"), os.Getenv("E2E_CODE"), os.Getenv("E2E_KEY")); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(conn)
	return &App{ctx: ctx, store: store}, store, conn
}

func logJSON(t *testing.T, label string, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	t.Logf("%s:\n%s", label, raw)
}

func count(t *testing.T, conn *sql.DB, q string) int64 {
	var n int64
	if err := conn.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestE2ESyncAll(t *testing.T) {
	app, store, conn := e2eApp(t)
	ctx := app.ctx

	t.Logf("before: clients=%d products=%d activeProducts=%d",
		count(t, conn, "select count(*) from clients"), count(t, conn, "select count(*) from products"),
		count(t, conn, "select count(*) from products where is_active=1"))

	start := time.Now()
	rep, err := app.ResyncAll(ResyncOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	logJSON(t, "dry run", rep.Entities)

	rep, err = app.ResyncAll(ResyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.RequiresConfirmation {
		t.Logf("first run asks for confirmation: %d products to deactivate", len(rep.PendingDeactivation.ProductBarcodes))
		rep, err = app.ResyncAll(ResyncOptions{ConfirmDeactivation: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	logJSON(t, "sync all", rep.Entities)
	logJSON(t, "full check", rep.FullCheck)
	t.Logf("sync all took %s", time.Since(start))

	// Drain the outbox against kashi, like the push loop would.
	client := newCentralClient(os.Getenv("E2E_URL"), os.Getenv("E2E_CODE"), os.Getenv("E2E_KEY"))
	drainStart := time.Now()
	for i := 0; i < 2000; i++ {
		before, _ := store.CountPendingSyncOutbox(ctx)
		if before == 0 {
			break
		}
		if err := app.runSyncOutboxOnce(ctx, client); err != nil {
			t.Fatal(err)
		}
		after, _ := store.CountPendingSyncOutbox(ctx)
		if after == before {
			// Remaining rows are on backoff.
			break
		}
	}
	pending, _ := store.CountSyncOutboxByStatus(ctx, "pending")
	failed, _ := store.CountSyncOutboxByStatus(ctx, "failed")
	synced, _ := store.CountSyncOutboxByStatus(ctx, "synced")
	t.Logf("drain took %s: synced=%d pending=%d failed=%d", time.Since(drainStart), synced, pending, failed)
	rows, _ := conn.Query(`select entity_type, last_error, count(*) from sync_outbox where status != 'synced' group by 1, 2 order by 3 desc limit 10`)
	for rows.Next() {
		var et, le string
		var n int
		rows.Scan(&et, &le, &n)
		t.Logf("  not synced: %s x%d: %s", et, n, le)
	}
	rows.Close()

	// Second full check after the push: everything should be current.
	app.fullCheckAttemptAt = time.Time{}
	fc, err := app.runFullCheckLocked(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	logJSON(t, "second full check", fc)

	t.Logf("after: clients=%d activeClients=%d products=%d activeProducts=%d clientsWithRemoteVersion=%d productsWithRemoteVersion=%d",
		count(t, conn, "select count(*) from clients"), count(t, conn, "select count(*) from clients where is_active=1"),
		count(t, conn, "select count(*) from products"), count(t, conn, "select count(*) from products where is_active=1"),
		count(t, conn, "select count(*) from clients where remote_updated_at is not null"),
		count(t, conn, "select count(*) from products where remote_updated_at is not null"))
}

// Changes made in kashi after the initial sync reach the till through the
// normal pull: a phone edit updates the same client in place, and a
// deactivated variant goes inactive locally.
func TestE2ELiveChanges(t *testing.T) {
	app, store, conn := e2eApp(t)
	ctx := app.ctx
	clientUID, newPhone, variantUID := os.Getenv("E2E_CLIENT_UID"), os.Getenv("E2E_NEW_PHONE"), os.Getenv("E2E_VARIANT_UID")

	before, err := store.GetClientByUID(ctx, sql.NullString{String: clientUID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	clientsBefore := count(t, conn, "select count(*) from clients")

	client := newCentralClient(os.Getenv("E2E_URL"), os.Getenv("E2E_CODE"), os.Getenv("E2E_KEY"))
	app.runSyncPullOnce(ctx, client)

	after, err := store.GetClientByUID(ctx, sql.NullString{String: clientUID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("client %d: phone %s -> %s, points %d -> %d", before.ID, before.Phone, after.Phone, before.TotalLoyaltyPoints, after.TotalLoyaltyPoints)
	if after.ID != before.ID || after.Phone != newPhone || after.TotalLoyaltyPoints != before.TotalLoyaltyPoints {
		t.Fatalf("phone change not applied in place: %+v", after)
	}
	if n := count(t, conn, "select count(*) from clients"); n != clientsBefore {
		t.Fatalf("client count changed %d -> %d (duplicate created?)", clientsBefore, n)
	}

	p, err := store.GetProductByUID(ctx, sql.NullString{String: variantUID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.IsActive != 0 {
		t.Fatalf("variant deactivated in kashi is still active locally: %+v", p)
	}
	t.Logf("product %d (%s) is inactive locally", p.ID, p.Barcode)
}
