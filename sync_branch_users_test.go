package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestBranchUserRosterSyncAndPinCommand(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "branch_users.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := store.CreateRole(ctx, db.CreateRoleParams{Name: "admin", Description: "Administrator"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRole(ctx, db.CreateRoleParams{Name: "cashier", Description: "Cashier"}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("entity") == "branch_users" {
			fmt.Fprint(w, `{"entity":"branch_users","items":[{"id":42,"username":"sara","role":"cashier","akuvox_user_id":"AKU-9","is_active":true}],"server_time":"2026-08-31T00:00:00Z"}`)
			return
		}
		fmt.Fprintf(w, `{"entity":%q,"items":[],"server_time":"2026-08-31T00:00:00Z"}`, r.URL.Query().Get("entity"))
	}))
	defer server.Close()

	app := &App{ctx: ctx, store: store, conn: database}
	client := newCentralClient(server.URL, "TEST01", "test-key")

	if err := app.pullEntityOnce(ctx, client, "branch_users"); err != nil {
		t.Fatalf("pull branch_users: %v", err)
	}

	u, err := store.GetUserByUsername(ctx, "sara")
	if err != nil {
		t.Fatalf("synced user not created: %v", err)
	}
	if !u.RemoteID.Valid || u.RemoteID.Int64 != 42 {
		t.Errorf("remote_id = %+v, want 42", u.RemoteID)
	}
	if u.AttendanceUserID != "AKU-9" {
		t.Errorf("attendance_user_id = %q, want AKU-9", u.AttendanceUserID)
	}
	if u.PasswordHash != "" {
		t.Errorf("synced user should have no PIN yet, got hash %q", u.PasswordHash)
	}

	// set_branch_user_pin sets the hash for the existing synced user.
	pinPayload, _ := json.Marshal(setBranchUserPinPayload{Username: "sara", Pin: "4821"})
	if err := app.executeSetBranchUserPinCommand(ctx, pinPayload); err != nil {
		t.Fatalf("set pin: %v", err)
	}
	u, _ = store.GetUserByUsername(ctx, "sara")
	if u.PasswordHash != hashPassword("4821") {
		t.Errorf("PIN not applied: hash = %q", u.PasswordHash)
	}

	// A second roster pull must not disturb the PIN or duplicate the user.
	if err := app.pullEntityOnce(ctx, client, "branch_users"); err != nil {
		t.Fatalf("second pull: %v", err)
	}
	all, _ := store.ListUsers(ctx)
	n := 0
	for _, x := range all {
		if x.Username == "sara" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected exactly one 'sara' user, got %d", n)
	}
	u, _ = store.GetUserByUsername(ctx, "sara")
	if u.PasswordHash != hashPassword("4821") {
		t.Errorf("roster re-sync clobbered the PIN: %q", u.PasswordHash)
	}

	// A PIN command for a not-yet-synced user creates it as a cashier.
	pinPayload, _ = json.Marshal(setBranchUserPinPayload{Username: "walkin", Pin: "0000"})
	if err := app.executeSetBranchUserPinCommand(ctx, pinPayload); err != nil {
		t.Fatalf("set pin for new user: %v", err)
	}
	if _, err := store.GetUserByUsername(ctx, "walkin"); err != nil {
		t.Errorf("PIN command should have created the user: %v", err)
	}
}
