package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// A new photo link for a loan comes back as the loan upload URL, built on
// the till's own central URL; kashi's refusal reaches the cashier as-is.
func TestRequestLoanUploadLink(t *testing.T) {
	var gotRef string
	kashi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/branch/loans/upload_link" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			ClientRef string `json:"client_ref"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotRef = body.ClientRef
		if body.ClientRef == "full" {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"this loan already has 3 photos"}`))
			return
		}
		w.Write([]byte(`{"upload_token":"abc123"}`))
	}))
	defer kashi.Close()

	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "photos.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, receipt_enabled, sales_inventory_id, central_api_url, branch_code
	) VALUES ('Test', 'key', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, ?, 'CN')`, kashi.URL); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}

	url, err := app.RequestLoanUploadLink("loan-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotRef != "loan-1" || url != kashi.URL+"/l/abc123" {
		t.Fatalf("ref %q, url %q", gotRef, url)
	}

	if _, err := app.RequestLoanUploadLink("full"); err == nil || !strings.Contains(err.Error(), "already has 3 photos") {
		t.Fatalf("err = %v, want kashi's message", err)
	}
}
