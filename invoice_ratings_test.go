package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// The invoices list gets central's poll answers for the codes it shows, unless
// the branch turned the ratings off; a central that fails gives no ratings.
func TestGetInvoiceRatings(t *testing.T) {
	var asked []string
	fail := false
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/branch/invoice_ratings" || fail {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Codes []string `json:"codes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		asked = body.Codes
		w.Write([]byte(`{"ratings":{"S-1":{"rating":5,"answer":"😍 Loved it"}}}`))
	}))
	defer central.Close()

	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "ratings.db")+"?_pragma=foreign_keys(ON)")
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
	) VALUES ('Test', 'key', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, ?, 'CN')`, central.URL); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}

	got := app.GetInvoiceRatings([]string{"S-1", " ", "S-2"})
	if len(asked) != 2 || got["S-1"].Rating != 5 || got["S-1"].Answer != "😍 Loved it" || len(got) != 1 {
		t.Fatalf("on by default: asked %v, got %v", asked, got)
	}

	fail = true
	if got := app.GetInvoiceRatings([]string{"S-1"}); len(got) != 0 {
		t.Fatalf("Central failing: %v, want none", got)
	}

	fail, asked = false, nil
	if err := app.setFeatureFlagBool(ctx, invoiceRatingsFlag, false); err != nil {
		t.Fatal(err)
	}
	if got := app.GetInvoiceRatings([]string{"S-1"}); len(got) != 0 || asked != nil {
		t.Fatalf("turned off: asked %v, got %v", asked, got)
	}
}
