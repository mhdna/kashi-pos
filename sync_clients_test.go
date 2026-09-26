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

// New clients (and products, per applySyncedProductVariant) default active;
// a client reappearing in a later pull must be reactivated even if a
// previous full-mirror sync had deactivated it locally.
func TestApplySyncedClient_ReactivatesDeactivatedClient(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)

	if err := applySyncedClient(ctx, store.Queries, syncedClient{Name: "Ali", Phone: "70123456"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeactivateClientsByPhone(ctx, []string{"70123456"}); err != nil {
		t.Fatal(err)
	}
	deactivated, err := store.GetClientByPhone(ctx, "70123456")
	if err != nil {
		t.Fatal(err)
	}
	if deactivated.IsActive != 0 {
		t.Fatalf("expected client deactivated, got is_active=%d", deactivated.IsActive)
	}

	// Client reappears in a fresh pull from kashi.
	if err := applySyncedClient(ctx, store.Queries, syncedClient{Name: "Ali", Phone: "70123456"}); err != nil {
		t.Fatal(err)
	}
	reactivated, err := store.GetClientByPhone(ctx, "70123456")
	if err != nil {
		t.Fatal(err)
	}
	if reactivated.IsActive != 1 {
		t.Fatalf("expected client reactivated after reappearing in pull, got is_active=%d", reactivated.IsActive)
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
	if err := store.DeactivateClientsByPhone(ctx, []string{"2"}); err != nil {
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
