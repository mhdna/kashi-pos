package main

import (
	"encoding/json"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func quickAccessPayload(t *testing.T, mode string, changedAt time.Time, uids ...string) json.RawMessage {
	t.Helper()
	type item struct {
		ProductUID string `json:"product_uid"`
		Position   int    `json:"position"`
		Color      string `json:"color"`
	}
	items := make([]item, len(uids))
	for i, uid := range uids {
		items[i] = item{ProductUID: uid, Position: i, Color: "#ff0000"}
	}
	raw, err := json.Marshal([]map[string]any{{"mode": mode, "mode_changed_at": changedAt, "items": items}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Pulling kashi's Quick Access stores its list and adopts its mode when
// kashi's choice is newer; in 'kashi' mode the panel shows kashi's cards
// (resolved by uid, unknown products skipped) and till-side edits refuse.
func TestQuickAccessKashiModeShowsCentralListReadOnly(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	product, err := store.GetProductByBarcode(app.ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	raw := quickAccessPayload(t, "kashi", time.Now(), product.Uid.String, "uid-this-till-does-not-have")
	if err := store.ExecTx(app.ctx, func(q *db.Queries) error {
		return applySyncedEntityBatch(app.ctx, q, "quick_access", raw)
	}); err != nil {
		t.Fatal(err)
	}

	cards, err := app.ListQuickAccessProducts()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0]["id"] != product.ID || cards[0]["quickAccessColor"] != "#ff0000" {
		t.Fatalf("kashi-mode cards = %v, want the one known product in kashi's colour", cards)
	}
	if err := app.AddQuickAccessProduct(product.ID); err != errQuickAccessManagedInKashi {
		t.Fatalf("AddQuickAccessProduct in kashi mode: err = %v, want errQuickAccessManagedInKashi", err)
	}
}

// The later flip wins: kashi's older choice must not undo a newer one made
// here (e.g. still waiting in the outbox).
func TestQuickAccessPullKeepsNewerLocalFlip(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	app.currentRole = "admin"

	kashiFlip := time.Now().Add(-time.Minute)
	if err := app.SetQuickAccessMode(quickAccessModeKashi); err != nil {
		t.Fatal(err)
	}
	if err := store.ExecTx(app.ctx, func(q *db.Queries) error {
		return applySyncedEntityBatch(app.ctx, q, "quick_access", quickAccessPayload(t, "till", kashiFlip))
	}); err != nil {
		t.Fatal(err)
	}
	if src, _ := app.GetQuickAccessSource(); src.Mode != quickAccessModeKashi {
		t.Fatalf("an older kashi choice overrode a newer local flip: mode = %q", src.Mode)
	}

	// A newer one from kashi does win.
	if err := store.ExecTx(app.ctx, func(q *db.Queries) error {
		return applySyncedEntityBatch(app.ctx, q, "quick_access", quickAccessPayload(t, "till", time.Now().Add(time.Minute)))
	}); err != nil {
		t.Fatal(err)
	}
	if src, _ := app.GetQuickAccessSource(); src.Mode != quickAccessModeTill {
		t.Fatalf("a newer kashi choice was ignored: mode = %q", src.Mode)
	}
}

// A flip made on the till goes to kashi through the outbox (so an offline
// flip still arrives), and only the latest flip is kept there.
func TestSetQuickAccessModeQueuesOnlyLatestFlip(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://kashi.invalid")

	if err := app.SetQuickAccessMode(quickAccessModeKashi); err == nil {
		t.Fatal("non-admin could flip the Quick Access source")
	}
	app.currentRole = "admin"
	for _, mode := range []string{quickAccessModeKashi, quickAccessModeTill, quickAccessModeKashi} {
		if err := app.SetQuickAccessMode(mode); err != nil {
			t.Fatal(err)
		}
	}

	due, err := store.ListDueSyncOutboxEntries(app.ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var flips []db.SyncOutbox
	for _, e := range due {
		if e.EntityType == "quick_access_mode" {
			flips = append(flips, e)
		}
	}
	if len(flips) != 1 {
		t.Fatalf("queued %d mode flips, want only the latest one", len(flips))
	}
	var payload struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(flips[0].PayloadJson), &payload); err != nil || payload.Mode != quickAccessModeKashi {
		t.Fatalf("queued flip = %s, want mode kashi", flips[0].PayloadJson)
	}
}
