package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestListProductsPageBoundsAndSearch(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestProducts(t, database, 150)

	first, err := app.ListProductsPage(1, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 5 || first.Total != 150 {
		t.Fatalf("first page has %d items and total %d, want 5 and 150", len(first.Items), first.Total)
	}
	if first.Items[0]["code"] != "SKU-000" || first.Items[4]["code"] != "SKU-004" {
		t.Fatalf("unexpected first page: %#v", first.Items)
	}

	second, err := app.ListProductsPage(2, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 5 || second.Items[0]["code"] != "SKU-005" {
		t.Fatalf("unexpected second page: %#v", second.Items)
	}

	defaultSize, err := app.ListProductsPage(1, 0, "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(defaultSize.Items) != 5 {
		t.Fatalf("default page has %d items, want 5", len(defaultSize.Items))
	}

	maxSize, err := app.ListProductsPage(1, 1_000, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(maxSize.Items) != 100 {
		t.Fatalf("capped page has %d items, want 100", len(maxSize.Items))
	}

	filtered, err := app.ListProductsPage(1, 5, " SKU-011 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.Total != 1 || filtered.Items[0]["code"] != "SKU-011" {
		t.Fatalf("unexpected filtered page: %#v (total %d)", filtered.Items, filtered.Total)
	}
}

func BenchmarkListProductsPage(b *testing.B) {
	app, database := newProductBenchmarkApp(b)
	defer database.Close()
	insertBenchmarkProducts(b, database, 10_000)

	b.Run("unfiltered", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.ListProductsPage(1, 5, ""); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("search", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.ListProductsPage(1, 5, "Product 09999"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func newProductTestApp(t *testing.T) (*App, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "products.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(context.Background(), database, "", false); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return &App{ctx: context.Background(), store: db.NewStore(database)}, database
}

func newProductBenchmarkApp(b *testing.B) (*App, *sql.DB) {
	b.Helper()
	database, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "products.db"))
	if err != nil {
		b.Fatal(err)
	}
	if err := applyMigrations(context.Background(), database, "", false); err != nil {
		database.Close()
		b.Fatal(err)
	}
	return &App{ctx: context.Background(), store: db.NewStore(database)}, database
}

func insertTestProducts(t *testing.T, database *sql.DB, count int) {
	t.Helper()
	if err := insertProducts(database, count); err != nil {
		t.Fatal(err)
	}
}

func insertBenchmarkProducts(b *testing.B, database *sql.DB, count int) {
	b.Helper()
	if err := insertProducts(database, count); err != nil {
		b.Fatal(err)
	}
}

func insertProducts(database *sql.DB, count int) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO products (barcode, code, name, description, is_active, price, discount)
		VALUES (?, ?, ?, '', 1, 100, 0)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for i := 0; i < count; i++ {
		if _, err := stmt.Exec(
			fmt.Sprintf("BAR-%06d", i),
			fmt.Sprintf("SKU-%03d", i),
			fmt.Sprintf("Product %05d", i),
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
