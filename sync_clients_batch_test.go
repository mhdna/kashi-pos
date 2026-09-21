package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	db "kashi-pos/db/sqlc"
)

func queueTestClientRows(t *testing.T, app *App, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		body, _ := json.Marshal(branchClientPayload{BranchClientID: id, Name: fmt.Sprintf("C%d", id), Phone: fmt.Sprintf("7000%04d", id)})
		if err := app.upsertOutbox(app.ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "client", ClientRef: "client-" + strconv.FormatInt(id, 10), PayloadJson: string(body),
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func outboxStatusByRef(t *testing.T, app *App) map[string]string {
	t.Helper()
	rows, err := app.store.ListDueSyncOutboxEntries(app.ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	pending := map[string]string{}
	for _, r := range rows {
		pending[r.ClientRef] = "pending"
	}
	return pending
}

func TestClientRowsDrainAsOneBatchPost(t *testing.T) {
	fc := newFakeCentral()
	server := httptest.NewServer(fc.handler())
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)

	// setUpTestInvoiceApp's own client isn't queued (no checkout), so these
	// are the only rows.
	queueTestClientRows(t, app, 101, 102, 103, 104, 105)

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	if fc.clientBatchCalls != 1 {
		t.Fatalf("expected exactly 1 clients_batch POST, got %d", fc.clientBatchCalls)
	}
	for _, id := range []string{"101", "102", "103", "104", "105"} {
		if fc.clientRefs[id] != 1 {
			t.Fatalf("client %s sent %d times, want 1", id, fc.clientRefs[id])
		}
	}
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 0 {
		t.Fatalf("expected 0 pending after a successful batch, got %d", n)
	}
}

func TestClientBatchSettlesEachRowFromItsOwnResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Clients []struct {
				BranchClientID int64 `json:"branch_client_id"`
			} `json:"clients"`
		}
		json.Unmarshal(body, &req)
		results := []map[string]any{}
		for _, c := range req.Clients {
			switch c.BranchClientID {
			case 201:
				results = append(results, map[string]any{"branch_client_id": c.BranchClientID, "status": "ok"})
			case 202:
				results = append(results, map[string]any{"branch_client_id": c.BranchClientID, "status": "invalid", "error": "bad"})
			case 203:
				results = append(results, map[string]any{"branch_client_id": c.BranchClientID, "status": "error", "error": "db down"})
			case 204:
				// no result at all for this one
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	queueTestClientRows(t, app, 201, 202, 203, 204)

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	// 201 ok -> synced; 202 invalid -> failed; 203 error and 204 (no
	// result) -> still pending.
	for status, want := range map[string]int64{"synced": 1, "failed": 1, "pending": 2} {
		got, err := store.CountSyncOutboxByStatus(app.ctx, status)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s rows: got %d, want %d", status, got, want)
		}
	}
	// The retryable ones are backed off, not immediately due again.
	if n := len(outboxStatusByRef(t, app)); n != 0 {
		t.Fatalf("expected retryable rows to be backed off, %d still due", n)
	}
}

func TestClientBatchFallsBackToPerRowWhenRouteMissing(t *testing.T) {
	fc := newFakeCentral()
	fc.clientBatchGone = true
	server := httptest.NewServer(fc.handler())
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	queueTestClientRows(t, app, 301, 302, 303)

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"301", "302", "303"} {
		if fc.clientRefs[id] != 1 {
			t.Fatalf("client %s sent %d times via fallback, want 1", id, fc.clientRefs[id])
		}
	}
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 0 {
		t.Fatalf("expected 0 pending after fallback, got %d", n)
	}
}

func TestClientBatchWholeCallFailureRetriesInsteadOfFailing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"too many clients in one batch"}`))
	}))
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	queueTestClientRows(t, app, 401, 402)

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	// A 400 on the envelope is the batch's fault, not each client's: the
	// rows must stay 'pending' (backed off), never be parked as 'failed'.
	if n, _ := store.CountPendingSyncOutbox(app.ctx); n != 2 {
		t.Fatalf("expected both rows still pending, got %d", n)
	}
}

func TestPushPassStopsWhenASaleIsQueuedMidDrain(t *testing.T) {
	var posts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		w.Write([]byte(`{"upload_token":""}`))
	}))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	for i := 0; i < 4; i++ {
		if err := app.upsertOutbox(app.ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "expense", ClientRef: fmt.Sprintf("exp-%d", i), PayloadJson: `{}`,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A kick already waiting == something was queued after this pass began.
	app.syncWorkerKick = make(chan struct{}, 1)
	app.syncWorkerKick <- struct{}{}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	if got := atomic.LoadInt32(&posts); got != 1 {
		t.Fatalf("expected the pass to stop after 1 row when preempted, sent %d", got)
	}
}
