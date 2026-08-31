package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"
)

func shiftRef(id int64) string { return "shift-" + strconv.FormatInt(id, 10) }

func decodeShiftClose(t *testing.T, entry db.SyncOutbox) branchShiftClosePayload {
	t.Helper()
	var payload branchShiftClosePayload
	if err := json.Unmarshal([]byte(entry.PayloadJson), &payload); err != nil {
		t.Fatalf("decode shift_close payload: %v", err)
	}
	return payload
}

// openShiftAndCloser returns the id of the fixture's open shift plus a
// freshly-created salesperson to close it as.
func openShiftAndCloser(t *testing.T, app *App, store *db.Store) (int64, db.Salesperson) {
	t.Helper()
	shift, err := store.GetOpenShift(app.ctx)
	if err != nil {
		t.Fatalf("get open shift: %v", err)
	}
	closer, err := store.CreateSalesperson(app.ctx, "Closer")
	if err != nil {
		t.Fatalf("create closing salesperson: %v", err)
	}
	return shift.ID, closer
}

func TestCloseShiftByIDQueuesShiftCloseSync(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	shiftID, closer := openShiftAndCloser(t, app, store)

	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "shift_close")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 shift_close entry, got %d", len(entries))
	}
	if entries[0].ClientRef != shiftRef(shiftID) {
		t.Fatalf("shift_close client_ref = %q, want %q", entries[0].ClientRef, shiftRef(shiftID))
	}
	payload := decodeShiftClose(t, entries[0])
	if payload.BranchShiftID != shiftID {
		t.Fatalf("payload branch_shift_id = %d, want %d", payload.BranchShiftID, shiftID)
	}
	if payload.ShiftRef != shiftRef(shiftID) {
		t.Fatalf("payload shift_ref = %q, want %q", payload.ShiftRef, shiftRef(shiftID))
	}
	if payload.ClosingPersonName != "Closer" {
		t.Fatalf("payload closing_person_name = %q, want Closer", payload.ClosingPersonName)
	}
	if payload.ClosedAt.IsZero() {
		t.Fatal("payload closed_at is the zero time; the close timestamp was not carried")
	}
	if payload.OccurredAt.IsZero() {
		t.Fatal("payload occurred_at is the zero time")
	}
}

func TestCloseShiftDoesNotQueueWhenSyncNotConfigured(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	shiftID, closer := openShiftAndCloser(t, app, store)

	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}

	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no outbox entries on an unconfigured install, got %d", count)
	}
}

func TestCloseShiftAlreadyClosedDoesNotDoubleQueue(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	shiftID, closer := openShiftAndCloser(t, app, store)

	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}
	// Second close of the same shift: CloseShift updates 0 rows, so the
	// whole transaction — including the outbox insert — must roll back.
	_, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}")
	if err == nil {
		t.Fatal("expected an error closing an already-closed shift")
	}

	entries := outboxByType(t, app, store, "shift_close")
	if len(entries) != 1 {
		t.Fatalf("expected still exactly 1 shift_close entry after a failed re-close, got %d", len(entries))
	}
}

func TestSyncWorkerRoutesShiftCloseToItsEndpoint(t *testing.T) {
	var mu sync.Mutex
	paths := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	shiftID, closer := openShiftAndCloser(t, app, store)
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/branch/shift_closes"] != 1 {
		t.Fatalf("POSTs to /branch/shift_closes = %d, want 1", paths["/branch/shift_closes"])
	}
	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending after a successful drain, got %d", count)
	}
}
