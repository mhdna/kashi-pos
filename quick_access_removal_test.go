package main

import (
	"context"
	"reflect"
	"testing"

	db "kashi-pos/db/sqlc"
)

// kashi removing a card must change what the till would show, and the
// whole-set snapshot must notice (that is what fires sync_applied).
func TestQuickAccessRemovalChangesTheSnapshot(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	apply := func(items string) (before, after any) {
		err := app.store.ExecTx(ctx, func(q *db.Queries) error {
			var err error
			before, _, err = wholeSetSnapshot(ctx, q, "quick_access")
			if err != nil {
				return err
			}
			if err := applySyncedEntityBatch(ctx, q, "quick_access", []byte(`[{"mode":"kashi","mode_changed_at":"2026-01-01T00:00:00Z","items":`+items+`}]`)); err != nil {
				return err
			}
			after, _, err = wholeSetSnapshot(ctx, q, "quick_access")
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return before, after
	}
	apply(`[{"product_uid":"a","position":0},{"product_uid":"b","position":1}]`)
	before, after := apply(`[{"product_uid":"a","position":0}]`)
	if reflect.DeepEqual(before, after) {
		t.Fatal("removing a card left the snapshot unchanged, so the screen is never told")
	}
	before, after = apply(`[]`)
	if reflect.DeepEqual(before, after) {
		t.Fatal("removing the last card left the snapshot unchanged")
	}
}
