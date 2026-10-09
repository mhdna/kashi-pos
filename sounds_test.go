package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// Sounds are off on a fresh till, and kashi's update_settings turns them on
// and off again through the same setter the Settings page uses (the settings
// report kashi sees reads the same soundsEnabled).
func TestSoundsOffByDefaultAndSetFromKashi(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "sounds.db")+"?_pragma=foreign_keys(ON)")
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
		rounding_mode, receipt_enabled, sales_inventory_id, central_api_url
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, 'http://kashi.test')`); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}

	if app.GetSoundsEnabled() {
		t.Fatal("sounds must be off until turned on")
	}

	for _, want := range []bool{true, false} {
		raw, _ := json.Marshal(map[string]bool{"sounds_enabled": want})
		if err := app.executeUpdateSettingsCommand(ctx, raw); err != nil {
			t.Fatalf("update_settings: %v", err)
		}
		if got := app.GetSoundsEnabled(); got != want {
			t.Fatalf("after kashi set sounds_enabled=%v, GetSoundsEnabled() = %v", want, got)
		}
	}
}
