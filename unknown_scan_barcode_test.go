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

// An unknown item scanned from a barcode the catalog didn't know sends that
// barcode with each of its units, so central's label reader makes the
// product under it; units added from the button send none.
func TestUnknownSessionSendsScannedBarcode(t *testing.T) {
	type slot struct {
		SlotRef   string  `json:"slot_ref"`
		LineRef   string  `json:"line_ref"`
		UnitPrice int64   `json:"unit_price"`
		Barcode   *string `json:"barcode"`
	}
	var got []slot
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/branch/unknown_sessions" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Slots []slot `json:"slots"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = body.Slots
		w.Write([]byte(`{"upload_token":"tok","complete":false,"slots":[]}`))
	}))
	defer central.Close()

	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "unknown.db")+"?_pragma=foreign_keys(ON)")
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

	units := `[
		{"label":"Unknown item $12.00","unitPriceCents":1200,"discount":0,"lineRef":"a","slotRef":"a:1","barcode":" 8000000092403 "},
		{"label":"Unknown item $12.00","unitPriceCents":1200,"discount":0,"lineRef":"a","slotRef":"a:2","barcode":"8000000092403"},
		{"label":"Unknown item $3.00","unitPriceCents":300,"discount":0,"lineRef":"b","slotRef":"b:1"}
	]`
	if _, err := app.StartUnknownItemSession(units); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("central got %d slots, want 3", len(got))
	}
	for i, want := range []string{"8000000092403", "8000000092403"} {
		if got[i].Barcode == nil || *got[i].Barcode != want {
			t.Fatalf("slot %d barcode = %v, want %q", i, got[i].Barcode, want)
		}
	}
	if got[2].Barcode != nil {
		t.Fatalf("a unit from the button sent barcode %q", *got[2].Barcode)
	}
	if got[0].UnitPrice != 1200 || got[0].LineRef != "a" || got[2].SlotRef != "b:1" {
		t.Fatalf("slots = %+v", got)
	}
}
