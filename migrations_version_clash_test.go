package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// Two migration files with one number break every till that has run either:
// the second can't record its version. Every number must be used once.
func TestMigrationVersionsAreUnique(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]string{}
	for _, m := range migrations {
		if other, ok := seen[m.version]; ok {
			t.Fatalf("%s and %s share version %d", other, m.name, m.version)
		}
		seen[m.version] = m.name
	}
}

func columnExists(t *testing.T, conn *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func tableExists(t *testing.T, conn *sql.DB, table string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// The reported case: a dev build recorded 149 for exchange_openings, then
// failed on 000149_invoice_discount_replaces_item_discounts ("UNIQUE
// constraint failed: schema_migrations.version"). Also a till on v1.1.187,
// whose 149 is the discount column, upgrading to a build with the openings.
// undoMigrationsAfter150 takes out what later migrations added, so a database
// built at today's version looks like one from before them (such a till never
// ran them) and they can run again.
func undoMigrationsAfter150(conn *sql.DB) error {
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS client_sources`,
		`ALTER TABLE clients DROP COLUMN source_uid`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func TestMigrationsHealTheVersion149Clash(t *testing.T) {
	for name, broken := range map[string]func(*sql.DB) error{
		"149 recorded for exchange_openings": func(conn *sql.DB) error {
			if err := undoMigrationsAfter150(conn); err != nil {
				return err
			}
			if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE version >= 149`); err != nil {
				return err
			}
			if _, err := conn.Exec(`ALTER TABLE invoice_discounts DROP COLUMN replaces_item_discounts`); err != nil {
				return err
			}
			_, err := conn.Exec(`INSERT INTO schema_migrations(version, name) VALUES (149, '000149_exchange_openings.up.sql')`)
			return err
		},
		"v1.1.187 till, 149 is the discount column": func(conn *sql.DB) error {
			if err := undoMigrationsAfter150(conn); err != nil {
				return err
			}
			if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE version >= 150`); err != nil {
				return err
			}
			_, err := conn.Exec(`DROP TABLE exchange_openings`)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "clash.db")+"?_pragma=foreign_keys(ON)")
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := applyMigrations(ctx, conn, "", false); err != nil {
				t.Fatal(err)
			}
			if err := broken(conn); err != nil {
				t.Fatal(err)
			}
			if err := applyMigrations(ctx, conn, "", false); err != nil {
				t.Fatalf("the till must open: %v", err)
			}
			if !columnExists(t, conn, "invoice_discounts", "replaces_item_discounts") {
				t.Fatal("invoice_discounts.replaces_item_discounts missing")
			}
			if !tableExists(t, conn, "exchange_openings") {
				t.Fatal("exchange_openings missing")
			}
			// Opening it again changes nothing and still works.
			if err := applyMigrations(ctx, conn, "", false); err != nil {
				t.Fatal(err)
			}
		})
	}
}
