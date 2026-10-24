package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func newReceiptLookApp(t *testing.T) *App {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "look.db")+"?_pragma=foreign_keys(ON)")
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
		rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: ctx, store: db.NewStore(database), conn: database}
}

func TestReceiptLookDefaultsToClassicColumns(t *testing.T) {
	app := newReceiptLookApp(t)
	theme, layout := app.receiptLook()
	if theme != "classic" || layout != "A" {
		t.Fatalf("default look = %s/%s, want classic/A", theme, layout)
	}
}

func TestSetReceiptLookNormalizesWhatItStores(t *testing.T) {
	app := newReceiptLookApp(t)
	if err := app.SetReceiptLook("Boutique", "b"); err != nil {
		t.Fatal(err)
	}
	if theme, layout := app.receiptLook(); theme != "boutique" || layout != "B" {
		t.Fatalf("look = %s/%s, want boutique/B", theme, layout)
	}
	// A layout the theme doesn't have falls back to the theme's first one;
	// an unknown theme falls back to the default.
	if err := app.SetReceiptLook("boutique", "C"); err != nil {
		t.Fatal(err)
	}
	if _, layout := app.receiptLook(); layout != "A" {
		t.Fatalf("layout = %s, want A", layout)
	}
	if err := app.SetReceiptLook("gone", "B"); err != nil {
		t.Fatal(err)
	}
	if theme, layout := app.receiptLook(); theme != "classic" || layout != "B" {
		t.Fatalf("look = %s/%s, want classic/B", theme, layout)
	}
}

func TestUpdateSettingsCommandSetsReceiptLook(t *testing.T) {
	app := newReceiptLookApp(t)
	apply := func(p updateSettingsCommandPayload) {
		t.Helper()
		raw, _ := json.Marshal(p)
		if err := app.executeUpdateSettingsCommand(app.ctx, raw); err != nil {
			t.Fatal(err)
		}
	}
	strp := func(s string) *string { return &s }

	apply(updateSettingsCommandPayload{ReceiptTheme: strp("supermarket"), ReceiptLayout: strp("C")})
	if theme, layout := app.receiptLook(); theme != "supermarket" || layout != "C" {
		t.Fatalf("look = %s/%s, want supermarket/C", theme, layout)
	}
	// A layout alone keeps the theme.
	apply(updateSettingsCommandPayload{ReceiptLayout: strp("B")})
	if theme, layout := app.receiptLook(); theme != "supermarket" || layout != "B" {
		t.Fatalf("look = %s/%s, want supermarket/B", theme, layout)
	}
	// A payload without either leaves it alone.
	apply(updateSettingsCommandPayload{})
	if theme, layout := app.receiptLook(); theme != "supermarket" || layout != "B" {
		t.Fatalf("look = %s/%s, want supermarket/B", theme, layout)
	}
}

func TestPreviewReceiptLookIsAPNGDataURL(t *testing.T) {
	app := newReceiptLookApp(t)
	url, err := app.PreviewReceiptLook("modern", "B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") || len(url) < 2000 {
		t.Fatalf("unexpected preview %q...", url[:min(len(url), 40)])
	}
}
