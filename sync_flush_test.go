package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// setUpFlushApp is setUpTestInvoiceApp with credentials filled in, so
// syncClient (which FlushSyncOutbox reads settings through) builds a client
// that actually authenticates.
func setUpFlushApp(t *testing.T, serverURL string) (*App, *db.Store) {
	t.Helper()
	app, store, _, _ := setUpTestInvoiceApp(t, serverURL)
	if _, err := app.conn.Exec(`UPDATE settings SET branch_code = 'TEST01', api_key = 'test-key'`); err != nil {
		t.Fatal(err)
	}
	return app, store
}

func queueOutbox(t *testing.T, store *db.Store, entityType string, n int) {
	t.Helper()
	for i := range n {
		if _, err := store.CreateSyncOutboxEntry(t.Context(), db.CreateSyncOutboxEntryParams{
			EntityType:  entityType,
			ClientRef:   fmt.Sprintf("%s-%d", entityType, i),
			PayloadJson: `{}`,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func pendingCount(t *testing.T, app *App, store *db.Store) int64 {
	t.Helper()
	n, err := store.CountSyncOutboxByStatus(app.ctx, "pending")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// A day's backlog after the connection comes back: the flush must ignore
// the link's and each row's backoff, send rows in parallel, and still never
// let a return reach kashi before the sales queued ahead of it.
func TestFlushSyncOutboxUploadsBacklogInParallelSalesFirst(t *testing.T) {
	var (
		mu          sync.Mutex
		order       []string
		inFlight    int32
		maxInFlight int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		n := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if n <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		mu.Lock()
		order = append(order, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, store := setUpFlushApp(t, server.URL)
	queueOutbox(t, store, "sales_invoice", 24)
	queueOutbox(t, store, "return_invoice", 6)
	queueOutbox(t, store, "expense", 4)
	// As left by a long outage: the link backing off, rows pushed out.
	app.pushLink.markDown(time.Now())
	if _, err := app.conn.Exec(`UPDATE sync_outbox SET next_attempt_at = datetime('now', '+1 hour')`); err != nil {
		t.Fatal(err)
	}

	progress, err := app.FlushSyncOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if progress.Remaining != 0 || progress.Done != 34 || progress.Total != 34 {
		t.Fatalf("progress = %+v, want 34/34 done, 0 remaining", progress)
	}
	if got := pendingCount(t, app, store); got != 0 {
		t.Fatalf("pending after flush = %d, want 0", got)
	}
	if maxInFlight < 2 {
		t.Fatalf("max in-flight requests = %d, want the flush to send in parallel", maxInFlight)
	}
	lastSale, firstReturn := -1, len(order)
	for i, path := range order {
		switch path {
		case "/branch/sales_invoices":
			lastSale = i
		case "/branch/return_invoices":
			firstReturn = min(firstReturn, i)
		}
	}
	if firstReturn < lastSale {
		t.Fatalf("a return reached kashi (#%d) before the last sale (#%d)", firstReturn, lastSale)
	}
	if !app.pushLink.isUp() {
		t.Fatal("expected a successful flush to mark the push link up")
	}
}

// A 5xx is about the row, not the connection: the row goes on backoff, and
// the flush re-arms it and tries again rather than leaving the cashier
// waiting on a timer.
func TestFlushSyncOutboxRetriesServerErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, store := setUpFlushApp(t, server.URL)
	queueOutbox(t, store, "sales_invoice", 1)

	if _, err := app.FlushSyncOutbox(); err != nil {
		t.Fatal(err)
	}
	if got := pendingCount(t, app, store); got != 0 {
		t.Fatalf("pending after flush = %d, want 0", got)
	}
	if calls != 2 {
		t.Fatalf("kashi saw %d requests, want 2 (one 503, one retry)", calls)
	}
}

func TestFlushSyncOutboxFailsFastWhenKashiUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer server.Close()

	app, store := setUpFlushApp(t, server.URL)
	queueOutbox(t, store, "sales_invoice", 3)

	if _, err := app.FlushSyncOutbox(); err == nil || !strings.Contains(err.Error(), "can't reach kashi") {
		t.Fatalf("err = %v, want a can't-reach-kashi error", err)
	}
	if got := pendingCount(t, app, store); got != 3 {
		t.Fatalf("pending = %d, want all 3 still queued", got)
	}
}

func TestCloseShiftSyncGate(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	shiftID, closer := openShiftAndCloser(t, app, store)
	queueOutbox(t, store, "sales_invoice", 2)

	// Off by default: closes as before.
	gate, err := app.GetCloseShiftSyncGate()
	if err != nil || gate.Required {
		t.Fatalf("gate = %+v, err = %v; want not required by default", gate, err)
	}

	if err := app.SetCloseShiftRequiresSyncEnabled(true); err != nil {
		t.Fatal(err)
	}
	gate, err = app.GetCloseShiftSyncGate()
	if err != nil || !gate.Required || gate.Pending != 2 {
		t.Fatalf("gate = %+v, err = %v; want required with 2 pending", gate, err)
	}
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err == nil ||
		!strings.HasPrefix(err.Error(), errCloseShiftSyncPending) {
		t.Fatalf("close err = %v, want the sync-pending refusal", err)
	}

	if err := app.BypassCloseShiftSyncGate(shiftID, "wrong", ""); err == nil {
		t.Fatal("expected a wrong admin password to be refused")
	}
	if err := app.BypassCloseShiftSyncGate(shiftID, defaultAdminPassword, "no internet"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatalf("close after bypass: %v", err)
	}
	if got := app.closeShiftSyncBypass.Load(); got != 0 {
		t.Fatalf("bypass still armed for shift %d after the close used it", got)
	}
}

// A till that doesn't sync at all has nothing to wait for.
func TestCloseShiftSyncGateIgnoredWithoutSync(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	shiftID, closer := openShiftAndCloser(t, app, store)
	if err := app.SetCloseShiftRequiresSyncEnabled(true); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}
}
