package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func TestSyncWorkerDrainsOutboxOnSuccess(t *testing.T) {
	var posts int32
	var sawInvoicePath atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		if r.URL.Path == "/branch/sales_invoices" {
			sawInvoicePath.Store(true)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invoice":{}}`))
	}))
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, server.URL)

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	// runSyncOutboxOnce directly (not the full tick) — this test is only
	// about the push/outbox side, and the fake server above doesn't know
	// how to answer a pull catch-up request.
	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	// invoice + its client both drain
	if posts != 2 {
		t.Fatalf("expected 2 POSTs to kashi (invoice + client), got %d", posts)
	}
	if !sawInvoicePath.Load() {
		t.Fatal("never POSTed to /branch/sales_invoices")
	}

	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending after a successful sync, got %d", count)
	}
}

func TestSyncWorkerRetriesWithBackoffOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, server.URL)

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	// Still pending (not synced), but not immediately due again — it should
	// have been pushed into the future by the backoff, not left at "now".
	due, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("expected the failed entry to not be immediately due again, got %d due", len(due))
	}

	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected both entries (invoice + client) still pending, got %d", count)
	}
}

// A kashi whose database is down answers its branch auth check with 401.
// That says nothing about the rows, so they must stay pending, not be
// parked as 'failed' until someone runs "sync everything".
func TestSyncWorkerRetriesOnUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"dial tcp: connection refused"}`))
	}))
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, server.URL)

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	failed, err := store.CountSyncOutboxByStatus(app.ctx, "failed")
	if err != nil {
		t.Fatal(err)
	}
	if failed != 0 {
		t.Fatalf("expected no rows parked as failed on a 401, got %d", failed)
	}
	if count, _ := store.CountPendingSyncOutbox(app.ctx); count != 2 {
		t.Fatalf("expected both entries (invoice + client) still pending, got %d", count)
	}
}

func TestIsRowRejection(t *testing.T) {
	cases := []struct {
		entity string
		status int
		want   bool
	}{
		{"sales_invoice", http.StatusBadRequest, true},
		{"sales_invoice", http.StatusUnauthorized, false},
		{"sales_invoice", http.StatusForbidden, false},
		{"sales_invoice", http.StatusNotFound, true},
		{"expense_cancel", http.StatusNotFound, false},
		{"loan_cancel", http.StatusNotFound, false},
		{"expense_from_central", http.StatusNotFound, false},
		{"expense_cancel", http.StatusBadRequest, true},
	}
	for _, c := range cases {
		err := &permanentError{status: c.status, msg: "x"}
		if got := isRowRejection(c.entity, err); got != c.want {
			t.Errorf("isRowRejection(%q, %d) = %v, want %v", c.entity, c.status, got, c.want)
		}
	}
}

func TestSyncWorkerNoopWhenNotConfigured(t *testing.T) {
	app, _, cashbox, client := setUpTestInvoiceApp(t, "")

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	// Nothing was queued (already covered by TestSalesInvoiceDoesNotQueueSyncWhenNotConfigured),
	// so this should just be a cheap no-op with no error and no HTTP calls.
	app.runSyncTick(app.ctx)
}

// While kashi pushes every change, the 15s pull tick only does the full
// pull (and the commands check) as a 5-minute backstop. Without that --
// no live session, or an older kashi that sends no hello -- it pulls on
// every tick, as before.
func TestPullTickBacksOffWhileKashiPushesEverything(t *testing.T) {
	fc := newFakeCentral()
	var fullPulls, commandChecks atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/branch/sync/changes" && r.URL.Query().Get("entity") == "currencies":
			fullPulls.Add(1) // currencies is first in every full pass
		case r.URL.Path == "/branch/commands/pending":
			commandChecks.Add(1)
		}
		fc.handler().ServeHTTP(w, r)
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")
	tick := func() (pulls, commands int32) {
		t.Helper()
		before, beforeCmd := fullPulls.Load(), commandChecks.Load()
		app.runPullTick(app.ctx)
		return fullPulls.Load() - before, commandChecks.Load() - beforeCmd
	}

	// No hello yet: every tick pulls.
	for range 2 {
		if pulls, commands := tick(); pulls != 1 || commands != 1 {
			t.Fatalf("without pushes: tick pulled %d times, checked commands %d times; want 1 and 1", pulls, commands)
		}
	}

	app.handleWSMessage(client, []byte(`{"type":"hello","features":["sync_changed"]}`))
	if pulls, commands := tick(); pulls != 0 || commands != 0 {
		t.Fatalf("right after a full pull, a pushed tick pulled %d / checked commands %d; want 0", pulls, commands)
	}

	// Backstop elapsed.
	app.lastFullPullAt = time.Now().Add(-syncPullBackstopInterval)
	if pulls, commands := tick(); pulls != 1 || commands != 1 {
		t.Fatalf("after the backstop interval: pulled %d / commands %d; want 1 and 1", pulls, commands)
	}

	// An older kashi's hello (or none) keeps full polling.
	app.handleWSMessage(client, []byte(`{"type":"hello","features":[]}`))
	if pulls, _ := tick(); pulls != 1 {
		t.Fatalf("kashi without sync_changed: tick pulled %d times, want 1", pulls)
	}
}

// Offline (pushLink backing off), the pull tick makes no requests at all.
func TestPullTickSkipsNetworkWhileKashiUnreachable(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	app.pushLink.markDown(time.Now())
	app.runPullTick(app.ctx)
	if n := requests.Load(); n != 0 {
		t.Fatalf("pull tick made %d requests while kashi was unreachable, want 0", n)
	}
}

// The settings report goes to kashi only when it changed, on a new session,
// or hourly -- not on every 15s tick as before.
func TestPushSettingsOnlySendsWhenSomethingChanged(t *testing.T) {
	var puts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/settings" {
			puts.Add(1)
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")
	push := func() int32 {
		t.Helper()
		before := puts.Load()
		if err := app.pushSettings(app.ctx, client); err != nil {
			t.Fatal(err)
		}
		return puts.Load() - before
	}

	if n := push(); n != 1 {
		t.Fatalf("first report: sent %d times, want 1", n)
	}
	if n := push(); n != 0 {
		t.Fatalf("unchanged settings: sent %d times, want 0", n)
	}

	if err := store.SetFeatureFlag(app.ctx, db.SetFeatureFlagParams{Key: clientRequiredFlag, Enabled: 0}); err != nil {
		t.Fatal(err)
	}
	if n := push(); n != 1 {
		t.Fatalf("after a setting changed: sent %d times, want 1", n)
	}

	app.settingsReport.forget() // what a new WebSocket session does
	if n := push(); n != 1 {
		t.Fatalf("on a new session: sent %d times, want 1", n)
	}

	app.settingsReport.sentAt = time.Now().Add(-settingsResendInterval)
	if n := push(); n != 1 {
		t.Fatalf("after the hourly refresh interval: sent %d times, want 1", n)
	}
}

// A flag the till treats as on until set must be reported to kashi as on
// too. Reported as off, kashi showed the product search button (and client
// requirement) off for a till that had them on, and a save from kashi would
// then have turned them off for real.
func TestPushSettingsReportsDefaultOnFlagsAsOn(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/settings" {
			body, _ = io.ReadAll(r.Body)
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	for _, key := range []string{productSearchEnabledFlag, clientRequiredFlag, scanByCodeFlag} {
		if _, err := app.conn.ExecContext(app.ctx, "DELETE FROM feature_flags WHERE key = ?", key); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.pushSettings(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}
	var got branchSettingsPayload
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode report %q: %v", body, err)
	}
	if !got.SearchButtonEnabled || !got.ClientRequired || !got.ScanByCodeEnabled {
		t.Fatalf("unset flags reported search=%v client=%v scanByCode=%v, want all true",
			got.SearchButtonEnabled, got.ClientRequired, got.ScanByCodeEnabled)
	}
}
