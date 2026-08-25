package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSyncWorkerDrainsOutboxOnSuccess(t *testing.T) {
	var posts int32
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		gotPath = r.URL.Path
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

	if posts != 1 {
		t.Fatalf("expected exactly 1 POST to kashi, got %d", posts)
	}
	if gotPath != "/branch/sales_invoices" {
		t.Fatalf("posted to %q, want /branch/sales_invoices", gotPath)
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
	if count != 1 {
		t.Fatalf("expected the entry to still be pending (never given up on), got %d", count)
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
