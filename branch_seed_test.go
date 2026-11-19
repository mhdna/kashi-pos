package main

import (
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func dbFilePath(t *testing.T, conn *sql.DB) string {
	t.Helper()
	var path string
	if err := conn.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil || path == "" {
		t.Fatalf("database path: %q %v", path, err)
	}
	return path
}

// The reported case: adding Test Branch on a till that already had Centro
// downloaded every product and client again. Now the new branch copies
// central's ones from Centro's database and its first sync fetches only
// what is missing or newer.
func TestNewBranchCopiesTheCatalog(t *testing.T) {
	fk := newFakeKashiCatalog()
	stamp := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	const ali = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	fk.clients[fcClientUID] = map[string]any{"uid": fcClientUID, "name": "Rana", "phone": "70555111", "is_active": true, "updated_at": stamp, "referral_code": "7EDXTH"}
	fk.clients[ali] = map[string]any{"uid": ali, "name": "Ali", "phone": "70555222", "is_active": true, "updated_at": stamp}
	fk.products[fcProductUID] = map[string]any{"uid": fcProductUID, "barcode": "KASHI-1", "code": "K1", "name": "Central Shirt",
		"price": map[string]any{"Int64": 1000, "Valid": true}, "is_active": true, "updated_at": stamp}
	srv := httptest.NewServer(fk.handler())
	defer srv.Close()
	cc := newCentralClient(srv.URL, "TEST01", "test-key")

	// Centro: synced with central, plus its own (points, a local client).
	centro, _, _, centroDB := setupQuoteTestApp(t, 0, "half_up", "LBP")
	if _, err := centro.runFullCheckLocked(centro.ctx, cc); err != nil {
		t.Fatal(err)
	}
	rana, _ := centro.store.GetClientByUID(centro.ctx, sql.NullString{String: fcClientUID, Valid: true})
	if _, err := centroDB.Exec(`UPDATE clients SET total_loyalty_points = 90, valid_loyalty_points = 90 WHERE id = ?`, rana.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := centro.store.CreateClient(centro.ctx, db.CreateClientParams{Name: "Only here", Phone: "70999999"}); err != nil {
		t.Fatal(err)
	}

	// Test Branch: a new empty database.
	tb, _, _, tbDB := setupQuoteTestApp(t, 0, "half_up", "LBP")
	products, clients, err := seedCatalogFromBranch(tb.ctx, tbDB, dbFilePath(t, centroDB))
	if err != nil {
		t.Fatal(err)
	}
	if products < 1 || clients != 2 {
		t.Fatalf("copied %d products, %d clients", products, clients)
	}
	got, err := tb.store.GetClientByUID(tb.ctx, sql.NullString{String: fcClientUID, Valid: true})
	if err != nil || got.Phone != "70555111" || got.ReferralCode != "7EDXTH" || !got.RemoteUpdatedAt.Valid {
		t.Fatalf("Rana: %+v %v", got, err)
	}
	if got.TotalLoyaltyPoints != 0 || got.ValidLoyaltyPoints != 0 {
		t.Fatal("Centro's points came along")
	}
	if _, err := tb.store.GetClientByPhone(tb.ctx, "70999999"); err != sql.ErrNoRows {
		t.Fatal("Centro's own client came along")
	}
	if _, err := tb.store.GetProductByUID(tb.ctx, sql.NullString{String: fcProductUID, Valid: true}); err != nil {
		t.Fatalf("the product: %v", err)
	}
	walkIns := 0
	_ = tbDB.QueryRow(`SELECT COUNT(*) FROM clients WHERE phone = ?`, walkInClientPhone).Scan(&walkIns)
	if walkIns > 1 {
		t.Fatal("two walk-in clients")
	}

	// The first sync fetches nothing: everything is current.
	fk.fetches = 0
	if _, err := tb.runFullCheckLocked(tb.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if fk.fetches != 0 {
		t.Fatalf("fetched %d times after the copy, want none", fk.fetches)
	}
	// One newer at central: only it is fetched.
	fk.clients[ali]["name"] = "Ali Hassan"
	fk.clients[ali]["updated_at"] = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := tb.runFullCheckLocked(tb.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if fk.fetches != 1 {
		t.Fatalf("fetched %d times, want once (Ali)", fk.fetches)
	}
	if a, _ := tb.store.GetClientByUID(tb.ctx, sql.NullString{String: ali, Valid: true}); a.Name != "Ali Hassan" {
		t.Fatalf("Ali: %q", a.Name)
	}
}
