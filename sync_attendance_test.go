package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// setUpTestAttendanceApp builds an App with both a.conn and a.store wired
// (unlike setUpTestInvoiceApp, which leaves a.conn nil) plus a settings row,
// since every attendance mutation talks to a.conn directly.
func setUpTestAttendanceApp(t *testing.T, centralAPIURL string) (*App, *db.Store) {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "attendance.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpdateCentralAPIURL(ctx, centralAPIURL); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: ctx, store: store, conn: database}, store
}

func makeSalesperson(t *testing.T, app *App, store *db.Store, name, attendanceUserID string) db.Salesperson {
	t.Helper()
	sp, err := store.CreateSalesperson(app.ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.conn.ExecContext(app.ctx, `UPDATE salespersons SET attendance_user_id = ? WHERE id = ?`, attendanceUserID, sp.ID); err != nil {
		t.Fatal(err)
	}
	sp.AttendanceUserID = attendanceUserID
	return sp
}

func insertAttendanceEvent(t *testing.T, app *App, sp db.Salesperson, date, tm string) int64 {
	t.Helper()
	res, err := app.conn.ExecContext(app.ctx, `INSERT INTO salesperson_attendance_events
		(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
		VALUES ('dev', ?, ?, ?, ?, ?, ?, ?, 'IN', 'Success')`,
		"evt-"+date+"-"+tm, sp.ID, sp.AttendanceUserID, sp.FullName, date, tm, date+"T"+tm+"Z")
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func decodeAttendanceChange(t *testing.T, entry db.SyncOutbox) branchAttendanceChangePayload {
	t.Helper()
	var payload branchAttendanceChangePayload
	if err := json.Unmarshal([]byte(entry.PayloadJson), &payload); err != nil {
		t.Fatalf("decode attendance_change payload: %v", err)
	}
	return payload
}

// fakeAkuvox stands in for the attendance device: it answers
// /api/doorlog/get with the given items and returns its host:port so it can
// be dropped straight into attendance_config.akuvox_ip.
func fakeAkuvox(t *testing.T, items []map[string]string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{"retcode": 0, "message": "ok", "data": map[string]any{"item": items}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

func TestSyncAttendanceQueuesBatchedEventsSync(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "http://localhost:9999")
	sp := makeSalesperson(t, app, store, "Ava", "U1")
	host := fakeAkuvox(t, []map[string]string{
		{"ID": "d1", "UserID": "U1", "Name": "Ava", "Date": "2026-08-20", "Time": "09:00:00", "Type": "IN", "Status": "Success"},
		{"ID": "d2", "UserID": "U1", "Name": "Ava", "Date": "2026-08-20", "Time": "18:00:00", "Type": "OUT", "Status": "Success"},
	})
	if err := app.UpdateAttendanceConfig(host, "", "", true); err != nil {
		t.Fatal(err)
	}

	if _, err := app.SyncAttendance("2026-08-20"); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "attendance_events")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 batched attendance_events entry, got %d", len(entries))
	}
	var payload branchAttendanceEventsPayload
	if err := json.Unmarshal([]byte(entries[0].PayloadJson), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 2 {
		t.Fatalf("expected 2 events in the batch, got %d", len(payload.Events))
	}
	if payload.Events[0].Ref != host+"|d1" || payload.Events[1].Ref != host+"|d2" {
		t.Fatalf("event refs = %q,%q, want %q,%q", payload.Events[0].Ref, payload.Events[1].Ref, host+"|d1", host+"|d2")
	}
	if payload.Events[0].SalespersonName != "Ava" || payload.Events[0].Type != "IN" {
		t.Fatalf("first event = %+v, want Ava/IN", payload.Events[0])
	}
	_ = sp
}

func TestSyncAttendanceQueuesNothingWhenNoNewEvents(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "http://localhost:9999")
	makeSalesperson(t, app, store, "Ava", "U1")
	host := fakeAkuvox(t, []map[string]string{
		{"ID": "d1", "UserID": "U1", "Name": "Ava", "Date": "2026-08-20", "Time": "09:00:00", "Type": "IN", "Status": "Success"},
	})
	if err := app.UpdateAttendanceConfig(host, "", "", true); err != nil {
		t.Fatal(err)
	}

	if _, err := app.SyncAttendance("2026-08-20"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SyncAttendance("2026-08-20"); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "attendance_events")
	if len(entries) != 1 {
		t.Fatalf("second sync imported nothing new, so it must not queue a second batch; got %d entries", len(entries))
	}
}

func TestSyncAttendanceNoQueueWhenNotConfigured(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "")
	makeSalesperson(t, app, store, "Ava", "U1")
	host := fakeAkuvox(t, []map[string]string{
		{"ID": "d1", "UserID": "U1", "Name": "Ava", "Date": "2026-08-20", "Time": "09:00:00", "Type": "IN", "Status": "Success"},
	})
	if err := app.UpdateAttendanceConfig(host, "", "", true); err != nil {
		t.Fatal(err)
	}

	if _, err := app.SyncAttendance("2026-08-20"); err != nil {
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

func TestSubmitAttendanceComplaintQueuesChangeSync(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "http://localhost:9999")
	sp := makeSalesperson(t, app, store, "Ben", "U2")
	eventID := insertAttendanceEvent(t, app, sp, "2026-08-21", "09:15:00")

	if err := app.SubmitAttendanceComplaint(sp.ID, "2026-08-21", eventID, "09:00", "IN", "replace", ""); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "attendance_change")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 attendance_change entry, got %d", len(entries))
	}
	payload := decodeAttendanceChange(t, entries[0])
	if payload.Kind != "complaint" {
		t.Fatalf("payload kind = %q, want complaint", payload.Kind)
	}
	if payload.Status != "pending" {
		t.Fatalf("payload status = %q, want pending", payload.Status)
	}
	if payload.SalespersonName != "Ben" || payload.RequestedTime != "09:00" || payload.RequestedType != "IN" || payload.Action != "replace" {
		t.Fatalf("payload = %+v, want Ben/09:00/IN/replace", payload)
	}
	if !strings.HasPrefix(payload.Ref, "acr-") || !strings.HasSuffix(payload.Ref, "-complaint") {
		t.Fatalf("payload ref = %q, want acr-<id>-complaint", payload.Ref)
	}
	if entries[0].ClientRef != payload.Ref {
		t.Fatalf("outbox client_ref %q != payload ref %q", entries[0].ClientRef, payload.Ref)
	}
}

func TestApproveAttendanceComplaintQueuesApprovalSync(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "http://localhost:9999")
	if err := app.UpdateAttendanceApproverPassword("secret"); err != nil {
		t.Fatal(err)
	}
	sp := makeSalesperson(t, app, store, "Cara", "U3")
	eventID := insertAttendanceEvent(t, app, sp, "2026-08-22", "09:20:00")
	if err := app.SubmitAttendanceComplaint(sp.ID, "2026-08-22", eventID, "09:00", "IN", "replace", ""); err != nil {
		t.Fatal(err)
	}
	requestID := lastChangeRequestID(t, app)

	if err := app.ApproveAttendanceComplaint(requestID, true, "secret"); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "attendance_change")
	// One from the complaint, one from the approval.
	if len(entries) != 2 {
		t.Fatalf("expected 2 attendance_change entries (complaint + approval), got %d", len(entries))
	}
	approval := decodeAttendanceChange(t, entries[1])
	if approval.Kind != "approval" {
		t.Fatalf("approval kind = %q, want approval", approval.Kind)
	}
	if approval.Status != "approved" {
		t.Fatalf("approval status = %q, want approved", approval.Status)
	}
	if approval.Ref != fmt.Sprintf("acr-%d-review", requestID) {
		t.Fatalf("approval ref = %q, want acr-%d-review", approval.Ref, requestID)
	}
	if approval.Actor != "manager approval" {
		t.Fatalf("approval actor = %q, want 'manager approval'", approval.Actor)
	}
}

func TestRejectAttendanceComplaintQueuesRejectedApproval(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "http://localhost:9999")
	if err := app.UpdateAttendanceApproverPassword("secret"); err != nil {
		t.Fatal(err)
	}
	sp := makeSalesperson(t, app, store, "Dan", "U4")
	eventID := insertAttendanceEvent(t, app, sp, "2026-08-23", "09:25:00")
	if err := app.SubmitAttendanceComplaint(sp.ID, "2026-08-23", eventID, "09:00", "IN", "replace", ""); err != nil {
		t.Fatal(err)
	}
	requestID := lastChangeRequestID(t, app)

	if err := app.ApproveAttendanceComplaint(requestID, false, "secret"); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "attendance_change")
	if len(entries) != 2 {
		t.Fatalf("expected 2 attendance_change entries, got %d", len(entries))
	}
	approval := decodeAttendanceChange(t, entries[1])
	if approval.Status != "rejected" {
		t.Fatalf("approval status = %q, want rejected", approval.Status)
	}
}

func TestAttendanceChangeNoQueueWhenNotConfigured(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "")
	sp := makeSalesperson(t, app, store, "Eve", "U5")
	eventID := insertAttendanceEvent(t, app, sp, "2026-08-24", "09:30:00")

	if err := app.SubmitAttendanceComplaint(sp.ID, "2026-08-24", eventID, "09:00", "IN", "replace", ""); err != nil {
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

func TestSyncWorkerRoutesAttendanceToItsEndpoints(t *testing.T) {
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

	app, store := setUpTestAttendanceApp(t, server.URL)
	if err := app.UpdateAttendanceApproverPassword("secret"); err != nil {
		t.Fatal(err)
	}
	sp := makeSalesperson(t, app, store, "Fay", "U6")
	host := fakeAkuvox(t, []map[string]string{
		{"ID": "d1", "UserID": "U6", "Name": "Fay", "Date": "2026-08-25", "Time": "09:00:00", "Type": "IN", "Status": "Success"},
	})
	if err := app.UpdateAttendanceConfig(host, "", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SyncAttendance("2026-08-25"); err != nil {
		t.Fatal(err)
	}
	eventID := insertAttendanceEvent(t, app, sp, "2026-08-25", "09:40:00")
	if err := app.SubmitAttendanceComplaint(sp.ID, "2026-08-25", eventID, "09:00", "IN", "replace", ""); err != nil {
		t.Fatal(err)
	}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/branch/attendance_events"] != 1 {
		t.Fatalf("POSTs to /branch/attendance_events = %d, want 1", paths["/branch/attendance_events"])
	}
	if paths["/branch/attendance_changes"] != 1 {
		t.Fatalf("POSTs to /branch/attendance_changes = %d, want 1", paths["/branch/attendance_changes"])
	}
	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending after a successful drain, got %d", count)
	}
}

func lastChangeRequestID(t *testing.T, app *App) int64 {
	t.Helper()
	var id int64
	if err := app.conn.QueryRowContext(app.ctx, `SELECT id FROM attendance_change_requests ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
