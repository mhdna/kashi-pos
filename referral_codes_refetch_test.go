package main

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// The reported case: Centro's till pulled central's referral codes while it
// was too old to know about them, so older customers had none and their
// reprints printed no code. After 000145 the next pull fetches the codes,
// and a reprint prints the customer's.
func TestOldTillGetsReferralCodesBack(t *testing.T) {
	fk := newFakeKashiCatalog()
	stamp := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	const withCode = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	fk.clients[fcClientUID] = map[string]any{"uid": fcClientUID, "name": "Rana", "phone": "70555111", "is_active": true, "updated_at": stamp, "referral_code": "7EDXTH"}
	fk.clients[withCode] = map[string]any{"uid": withCode, "name": "Ali", "phone": "70555222", "is_active": true, "updated_at": stamp, "referral_code": "HQ4XJD"}
	srv := httptest.NewServer(fk.handler())
	defer srv.Close()
	server := srv.URL

	app, _, _, database := setupQuoteTestApp(t, 0, "half_up", "LBP")
	store := app.store
	ctx := context.Background()
	cc := newCentralClient(server, "TEST01", "test-key")
	if _, err := database.Exec(`INSERT OR REPLACE INTO referral_program (id, enabled, reward_percent) VALUES (1, 1, 15)`); err != nil {
		t.Fatal(err)
	}

	// The old till: both rows at central's version, Rana's without a code
	// (it didn't read them yet), and the clients cursor past them.
	if _, err := app.runFullCheckLocked(ctx, cc); err != nil {
		t.Fatal(err)
	}
	rana, err := store.GetClientByUID(ctx, sql.NullString{String: fcClientUID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE clients SET referral_code = '' WHERE id = ?`, rana.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSyncCursor(ctx, db.SetSyncCursorParams{Entity: "clients", Cursor: stamp}); err != nil {
		t.Fatal(err)
	}
	if code, _ := app.receiptReferral(rana.ID, receiptTexts{}); code != "" {
		t.Fatalf("setup: Rana already has %q", code)
	}
	// Before the fix a check finds nothing to fetch: the versions match.
	fk.fetches = 0
	if _, err := app.runFullCheckLocked(ctx, cc); err != nil {
		t.Fatal(err)
	}
	if fk.fetches != 0 {
		t.Fatalf("setup: a check fetched %d times", fk.fetches)
	}

	migration, err := os.ReadFile("db/migrations/000145_refetch_referral_codes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if c, err := store.GetSyncCursor(ctx, "clients"); err == nil && c != "" {
		t.Fatalf("clients cursor kept: %q", c)
	}

	// The next pull: the empty cursor runs the check, which fetches Rana only.
	if _, err := app.seedCursorFromLocalCatalog(ctx, cc, "clients"); err != nil {
		t.Fatal(err)
	}
	if code, note := app.receiptReferral(rana.ID, receiptTexts{}); code != "7EDXTH" || note == "" {
		t.Fatalf("Rana's receipt: code %q, note %q", code, note)
	}
	ali, _ := store.GetClientByUID(ctx, sql.NullString{String: withCode, Valid: true})
	if ali.ReferralCode != "HQ4XJD" {
		t.Fatalf("Ali's code: %q", ali.ReferralCode)
	}
	if fk.fetches != 1 {
		t.Fatalf("fetched %d times, want once (only the clients missing a code)", fk.fetches)
	}
}
