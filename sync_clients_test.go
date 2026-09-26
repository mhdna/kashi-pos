package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func setUpTestClientDB(t *testing.T) *db.Store {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "clients.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	return db.NewStore(database)
}

func boolPtr(b bool) *bool { return &b }

func mustClientByUID(t *testing.T, store *db.Store, uid string) db.Client {
	t.Helper()
	c, err := store.GetClientByUID(context.Background(), sql.NullString{String: uid, Valid: true})
	if err != nil {
		t.Fatalf("client %s: %v", uid, err)
	}
	return c
}

// kashi's is_active is copied down: a client kashi deactivates goes inactive
// here, and comes back when kashi reactivates it.
func TestApplySyncedClient_CopiesActiveFlag(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	const uid = "11111111-1111-4111-8111-111111111111"

	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: uid, Name: "Ali", Phone: "70123456", IsActive: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if c := mustClientByUID(t, store, uid); c.IsActive != 0 {
		t.Fatalf("expected inactive, got is_active=%d", c.IsActive)
	}
	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: uid, Name: "Ali", Phone: "70123456", IsActive: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if c := mustClientByUID(t, store, uid); c.IsActive != 1 {
		t.Fatalf("expected reactivated, got is_active=%d", c.IsActive)
	}
}

// A phone edited in kashi updates the same local row (matched by uid) and
// keeps its loyalty points, instead of creating a second client.
func TestApplySyncedClient_PhoneChangeUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	const uid = "22222222-2222-4222-8222-222222222222"

	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: uid, Name: "Ali", Phone: "70123456"}); err != nil {
		t.Fatal(err)
	}
	before := mustClientByUID(t, store, uid)
	if err := store.AddClientLoyaltyPoints(ctx, db.AddClientLoyaltyPointsParams{TotalLoyaltyPoints: 5, ValidLoyaltyPoints: 5, ID: before.ID}); err != nil {
		t.Fatal(err)
	}

	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: uid, Name: "Ali", Phone: "71999999"}); err != nil {
		t.Fatal(err)
	}
	after := mustClientByUID(t, store, uid)
	if after.ID != before.ID || after.Phone != "71999999" || after.TotalLoyaltyPoints != 5 {
		t.Fatalf("expected same row with new phone and points kept, got %+v (was id %d)", after, before.ID)
	}
	if _, err := store.GetClientByPhone(ctx, "70123456"); err != sql.ErrNoRows {
		t.Fatalf("old phone should be gone, got err=%v", err)
	}
}

// A local client created at the till adopts kashi's uid when kashi reports
// the same phone, rather than being duplicated.
func TestApplySyncedClient_AdoptsUIDByPhone(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	const uid = "33333333-3333-4333-8333-333333333333"

	local, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Ali", Phone: "70123456"})
	if err != nil {
		t.Fatal(err)
	}
	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: uid, Name: "Ali K", Phone: "70123456"}); err != nil {
		t.Fatal(err)
	}
	c := mustClientByUID(t, store, uid)
	if c.ID != local.ID || c.Name != "Ali K" {
		t.Fatalf("expected local row %d to adopt uid and name, got %+v", local.ID, c)
	}
}

// kashi answering a push with another client's uid means the pushed row is
// a duplicate of a client the till already holds: its invoices and points
// move onto that client and the duplicate is removed.
func TestAdoptCanonicalClientUID_MergesDuplicate(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	const canonical = "44444444-4444-4444-8444-444444444444"

	if err := applySyncedClient(ctx, store.Queries, syncedClient{UID: canonical, Name: "Ali", Phone: "70123456"}); err != nil {
		t.Fatal(err)
	}
	keep := mustClientByUID(t, store, canonical)
	dup, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Ali", Phone: "71000000", TotalLoyaltyPoints: 3, ValidLoyaltyPoints: 3})
	if err != nil {
		t.Fatal(err)
	}

	if err := adoptCanonicalClientUID(ctx, store.Queries, dup.ID, canonical); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetClientByID(ctx, dup.ID); err != sql.ErrNoRows {
		t.Fatalf("duplicate should be merged away, got err=%v", err)
	}
	merged := mustClientByUID(t, store, canonical)
	if merged.ID != keep.ID || merged.TotalLoyaltyPoints != 3 {
		t.Fatalf("expected points folded into %d, got %+v", keep.ID, merged)
	}
}

func TestListActiveClients_ExcludesDeactivated(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)

	if _, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Active", Phone: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Inactive", Phone: "2"}); err != nil {
		t.Fatal(err)
	}
	inactive, err := store.GetClientByPhone(ctx, "2")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetClientActive(ctx, db.SetClientActiveParams{IsActive: 0, ID: inactive.ID}); err != nil {
		t.Fatal(err)
	}

	active, err := store.ListActiveClients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Phone != "1" {
		t.Fatalf("expected only the active client, got %+v", active)
	}
}

func TestDeactivateProductsByBarcode(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)

	created, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "BC-1", Code: "C-1", Name: "Widget", IsActive: 1, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeactivateProductsByBarcode(ctx, []string{"BC-1"}); err != nil {
		t.Fatal(err)
	}
	updated, err := store.GetProductByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IsActive != 0 {
		t.Fatalf("expected product deactivated, got is_active=%d", updated.IsActive)
	}
}
