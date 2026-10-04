package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func queueTestInvoices(t *testing.T, store *db.Store, n int) []db.SyncOutbox {
	t.Helper()
	entries := make([]db.SyncOutbox, n)
	for i := range entries {
		entry, err := store.CreateSyncOutboxEntry(t.Context(), db.CreateSyncOutboxEntryParams{
			EntityType:  "sales_invoice",
			ClientRef:   fmt.Sprintf("test-invoice-%d", i),
			PayloadJson: `{}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = entry
	}
	return entries
}

// The offline half of the "44 invoices took ten minutes" bug: a pass with
// no network used to try every row (each sitting through the 10s timeout)
// and push each one's own retry further out, up to an hour. Now the pass
// stops at the first network failure, and the rows stay due with their
// attempt counts untouched -- only the connection backs off.
func TestPushPassStopsAtFirstNetworkErrorAndKeepsRowsDue(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		// Drop the connection without answering: what the client sees when
		// the shop's internet is down.
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	queueTestInvoices(t, store, 3)

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected the pass to stop after 1 failed request, made %d", got)
	}
	due, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 3 {
		t.Fatalf("expected all 3 invoices still due, got %d", len(due))
	}
	for _, entry := range due {
		if entry.AttemptCount != 0 {
			t.Fatalf("entry %d: attempt_count = %d, want 0 (a network error isn't the row's fault)", entry.ID, entry.AttemptCount)
		}
	}
	if !app.pushLink.waiting(time.Now()) {
		t.Fatal("expected the push link to be backing off after a network error")
	}
}

// The reconnect half: invoices an older build (or a long outage) had backed
// off by up to an hour must all go up on the first pass that reaches kashi,
// not each on its own timer.
func TestPushTickReleasesBackedOffRowsWhenKashiIsReachable(t *testing.T) {
	var posts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/branch/health":
			w.WriteHeader(http.StatusOK)
		case "/branch/sales_invoices":
			atomic.AddInt32(&posts, 1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"invoice":{}}`))
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	inAnHour := time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05")
	for _, entry := range queueTestInvoices(t, store, 3) {
		if err := store.MarkSyncOutboxAttemptFailed(app.ctx, db.MarkSyncOutboxAttemptFailedParams{
			LastError: "offline", NextAttemptAt: inAnHour, ID: entry.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	app.runPushTick(app.ctx)

	if got := atomic.LoadInt32(&posts); got != 3 {
		t.Fatalf("expected all 3 backed-off invoices posted in one tick, got %d", got)
	}
	pending, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("expected nothing pending, got %d", pending)
	}
	if !app.pushLink.isUp() {
		t.Fatal("expected the push link to be up after reaching kashi")
	}
}
