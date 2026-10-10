package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestReportInvoiceSalespersonsOnce(t *testing.T) {
	ctx := context.Background()
	// Foreign keys off: the invoices below skip their cashbox/shift/client rows.
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "sp.db"))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	app := &App{ctx: ctx, store: store, conn: database}

	fatima, _ := store.CreateSalesperson(ctx, "Fatima D.")
	local, _ := store.CreateSalesperson(ctx, "Only here")
	if _, err := database.Exec(`UPDATE salespersons SET remote_id = 41 WHERE id = ?`, fatima.ID); err != nil {
		t.Fatal(err)
	}
	for i, sp := range []int64{fatima.ID, fatima.ID, local.ID, 0} {
		if _, err := database.Exec(`INSERT INTO invoices (cashbox_id, shift_id, invoice_code, invoice_index, year, client_id, inventory_id, discount, subtotal, discounted_total, grand_total, salesperson_id)
			VALUES (1, 1, ?, ?, 2026, 1, 1, 0, 100, 100, 100, ?)`, []string{"SA/1", "SA/2", "SA/3", "SA/4"}[i], i+1, sp); err != nil {
			t.Fatal(err)
		}
	}

	if got := salespersonRemoteIDByName(ctx, store.Queries, "Fatima D."); got != 41 {
		t.Errorf("remote id by name = %d, want 41", got)
	}
	if got := salespersonRemoteID(ctx, store.Queries, local.ID); got != 0 {
		t.Errorf("remote id of a till-only salesperson = %d, want 0", got)
	}

	var mu sync.Mutex
	var posts []map[string][]invoiceSalespersonReport
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string][]invoiceSalespersonReport
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		posts = append(posts, body)
		mu.Unlock()
		w.Write([]byte(`{"updated":2}`))
	}))
	defer server.Close()
	client := newCentralClient(server.URL, "TEST01", "test-key")

	app.reportInvoiceSalespersons(ctx, client)
	app.reportInvoiceSalespersons(ctx, client) // already reported: no second request

	if len(posts) != 1 {
		t.Fatalf("reports sent = %d, want 1", len(posts))
	}
	items := posts[0]["items"]
	if len(items) != 2 || items[0].BranchInvoiceCode != "SA/1" || items[0].SalespersonID != 41 || items[1].BranchInvoiceCode != "SA/2" {
		t.Errorf("reported %+v, want SA/1 and SA/2 as 41", items)
	}
	settings, _ := store.GetSettings(ctx)
	if !settings.SalespersonIdsReportedAt.Valid {
		t.Error("not marked as reported")
	}
}
