package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"
)

// fakeCentral is a minimal stand-in for kashi: it accepts invoice/client
// pushes, remembers which refs it has seen, and answers reconcile against
// that set. Pull/command/settings endpoints return empty so a full sync
// tick can run against it.
type fakeCentral struct {
	mu             sync.Mutex
	invoiceRefs    map[string]int
	clientRefs     map[string]int
	productReports int
}

func newFakeCentral() *fakeCentral {
	return &fakeCentral{invoiceRefs: map[string]int{}, clientRefs: map[string]int{}}
}

func (f *fakeCentral) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		body, _ := io.ReadAll(r.Body)

		switch {
		case r.URL.Path == "/branch/sales_invoices" || r.URL.Path == "/branch/return_invoices":
			var p map[string]any
			json.Unmarshal(body, &p)
			ref, _ := p["client_ref"].(string)
			f.invoiceRefs[ref]++
			w.Write([]byte(`{"invoice":{}}`))
		case r.URL.Path == "/branch/clients":
			var p map[string]any
			json.Unmarshal(body, &p)
			if id, ok := p["branch_client_id"].(float64); ok {
				f.clientRefs[strconv.FormatInt(int64(id), 10)]++
			}
			w.Write([]byte(`{"client":{}}`))
		case r.URL.Path == "/branch/sync/reconcile":
			var req struct {
				Entity string   `json:"entity"`
				Refs   []string `json:"refs"`
			}
			json.Unmarshal(body, &req)
			seen := f.invoiceRefs
			if req.Entity == "clients" {
				seen = f.clientRefs
			}
			missing := []string{}
			for _, ref := range req.Refs {
				if req.Entity == "products" || seen[ref] == 0 {
					missing = append(missing, ref)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"missing": missing})
		case r.URL.Path == "/branch/products/report":
			var req struct {
				Products []map[string]any `json:"products"`
			}
			json.Unmarshal(body, &req)
			f.productReports += len(req.Products)
			json.NewEncoder(w).Encode(map[string]any{"staged": len(req.Products)})
		case r.URL.Path == "/branch/sync/changes":
			json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "server_time": "2020-01-01T00:00:00Z", "next_cursor": ""})
		case r.URL.Path == "/branch/commands/pending":
			w.Write([]byte(`{"commands":[]}`))
		case r.URL.Path == "/branch/settings":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	})
}

func TestResyncAllBackfillsInvoicesAndClientsExactlyOnce(t *testing.T) {
	fc := newFakeCentral()
	server := httptest.NewServer(fc.handler())
	defer server.Close()

	// Seed while sync is OFF: nothing is queued.
	app, store, cashbox, client := setUpTestInvoiceApp(t, "")
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	for i := 0; i < 3; i++ {
		if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 0 {
		t.Fatalf("expected 0 queued while sync off, got %d", n)
	}

	// Point at the fake server and sync everything.
	if err := store.UpdateCentralAPIURL(app.ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	rep, err := app.ResyncAll(ResyncOptions{})
	if err != nil {
		t.Fatalf("ResyncAll: %v", err)
	}

	// Drain the outbox the resync armed.
	cc := newCentralClient(server.URL, "TEST01", "test-key")
	for i := 0; i < 5; i++ {
		if err := app.runSyncOutboxOnce(app.ctx, cc); err != nil {
			t.Fatal(err)
		}
	}

	if got := len(fc.invoiceRefs); got != 3 {
		t.Fatalf("central received %d distinct invoices, want 3", got)
	}
	for ref, count := range fc.invoiceRefs {
		if count != 1 {
			t.Fatalf("invoice %s pushed %d times, want exactly 1", ref, count)
		}
	}
	if fc.clientRefs[strconv.FormatInt(client.ID, 10)] != 1 {
		t.Fatalf("client pushed %d times, want 1", fc.clientRefs[strconv.FormatInt(client.ID, 10)])
	}

	var invRep *ResyncEntityReport
	for i := range rep.Entities {
		if rep.Entities[i].Entity == "invoices" {
			invRep = &rep.Entities[i]
		}
	}
	if invRep == nil || invRep.Missing != 3 || invRep.Queued != 3 {
		t.Fatalf("invoice report = %+v, want Missing=3 Queued=3", invRep)
	}

	// Second run: central already has everything, nothing new is queued.
	fc.mu.Lock()
	before := len(fc.invoiceRefs)
	fc.mu.Unlock()
	rep2, err := app.ResyncAll(ResyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		app.runSyncOutboxOnce(app.ctx, cc)
	}
	if len(fc.invoiceRefs) != before {
		t.Fatalf("second resync pushed new invoices: %d -> %d", before, len(fc.invoiceRefs))
	}
	for _, e := range rep2.Entities {
		if e.Entity == "invoices" && e.Queued != 0 {
			t.Fatalf("second resync queued %d invoices, want 0", e.Queued)
		}
	}
}

func TestResyncAllDryRunQueuesNothing(t *testing.T) {
	fc := newFakeCentral()
	server := httptest.NewServer(fc.handler())
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, "")
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCentralAPIURL(app.ctx, server.URL); err != nil {
		t.Fatal(err)
	}

	rep, err := app.ResyncAll(ResyncOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 0 {
		t.Fatalf("dry run queued %d rows, want 0", n)
	}
	sawInvoices := false
	for _, e := range rep.Entities {
		if e.Entity == "invoices" {
			sawInvoices = true
			if e.Missing != 1 || e.Queued != 0 {
				t.Fatalf("dry-run invoice report = %+v, want Missing=1 Queued=0", e)
			}
		}
	}
	if !sawInvoices {
		t.Fatal("no invoice entity in dry-run report")
	}
}

func TestPermanentRejectionMovesOutboxRowToFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"nope"}`))
	}))
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, server.URL)
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	cc := newCentralClient(server.URL, "TEST01", "test-key")
	if err := app.runSyncOutboxOnce(app.ctx, cc); err != nil {
		t.Fatal(err)
	}

	if n, _ := store.CountSyncOutboxByStatus(app.ctx, "failed"); n != 1 {
		t.Fatalf("expected 1 failed row after a 400, got %d", n)
	}
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 0 {
		t.Fatalf("expected 0 pending after a 400 (row parked), got %d", n)
	}
	due, _ := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if len(due) != 0 {
		t.Fatalf("failed row should not be re-selected, got %d due", len(due))
	}
}

func TestUnknownEntityTypeMarkedFailedNotSpinning(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://unused.example")
	if err := store.ExecTx(app.ctx, func(q *db.Queries) error {
		_, err := q.CreateSyncOutboxEntry(app.ctx, db.CreateSyncOutboxEntryParams{
			EntityType: "bogus_kind", ClientRef: "bogus-1", PayloadJson: "{}",
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	cc := newCentralClient("http://unused.example", "TEST01", "test-key")
	if err := app.runSyncOutboxOnce(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.CountSyncOutboxByStatus(app.ctx, "failed"); n != 1 {
		t.Fatalf("unknown entity_type should be marked failed, got %d failed", n)
	}
}
