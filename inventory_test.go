package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// Each product on the page gets the other branches' rows for its variant,
// and a product no branch carries gets an empty list.
func TestListInventoryCursorAttachesBranchStock(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "inventory.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}
	if _, err := database.Exec(`
		INSERT INTO products (uid, barcode, code, name, price) VALUES
			('u-shirt', '111', 'SH1', 'Shirt', 0),
			('u-hat', '222', 'HT1', 'Hat', 0);
		INSERT INTO branch_stock (branch_code, variant_uid, barcode, quantity, price_cents) VALUES
			('CN', 'u-shirt', '111', 3, 1500),
			('SB', 'u-shirt', '111', 2, 1600);`); err != nil {
		t.Fatal(err)
	}

	page, err := app.ListInventoryCursor(0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(page.Items))
	}
	byName := map[string]map[string]any{}
	for _, item := range page.Items {
		byName[item["name"].(string)] = item
	}
	shirt := byName["Shirt"]["branches"].([]map[string]any)
	if len(shirt) != 2 || shirt[0]["branchCode"] != "CN" || byName["Shirt"]["otherStock"] != int64(5) {
		t.Fatalf("shirt branches = %v, otherStock = %v", shirt, byName["Shirt"]["otherStock"])
	}
	if hat := byName["Hat"]["branches"].([]map[string]any); len(hat) != 0 {
		t.Fatalf("hat branches = %v, want none", hat)
	}
}
