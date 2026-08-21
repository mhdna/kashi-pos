package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyMigrationsFreshDatabase(t *testing.T) {
	conn := openTestDatabase(t)
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	assertMigrationVersion(t, conn, 65)
	assertTableExists(t, conn, "settings")
	assertTableExists(t, conn, "feature_flags")
	assertTableExists(t, conn, "schema_migrations")
	assertTableExists(t, conn, "invoice_payment_entries")
	assertTableExists(t, conn, "quick_access_items")
	assertColumnExists(t, conn, "invoice_products", "salesperson_id")
	assertColumnExists(t, conn, "invoice_products", "note")
	assertIndexExists(t, conn, "idx_clients_phone", "clients")
	assertIndexExists(t, conn, "idx_products_barcode", "products")
	assertFeatureFlagEnabled(t, conn, "auto_close_stale_shifts", true)
}

func TestApplyMigrationsUpgradesLegacyDatabase(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "database")
	conn, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.version > 16 {
			break
		}
		if _, err := conn.Exec(migration.sql); err != nil {
			t.Fatalf("create legacy schema at %s: %v", migration.name, err)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	conn, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := applyMigrations(context.Background(), conn, databasePath, true); err != nil {
		t.Fatalf("upgrade legacy database: %v", err)
	}
	assertMigrationVersion(t, conn, 65)
	assertTableExists(t, conn, "schema_migrations")
	assertTableExists(t, conn, "invoice_payment_entries")
	assertTableExists(t, conn, "quick_access_items")
	assertColumnExists(t, conn, "invoice_products", "salesperson_id")
	assertColumnExists(t, conn, "invoice_products", "note")
	assertIndexExists(t, conn, "idx_clients_phone", "clients")
	assertIndexExists(t, conn, "idx_products_barcode", "products")
	// Migration 53 (fresh installs only) must not flip auto-close on for an
	// upgrading install, but migration 56 explicitly turns it on for
	// everyone (a later, deliberate product decision), so it ends up
	// enabled here too - just via 56, not 53.
	assertFeatureFlagEnabled(t, conn, "auto_close_stale_shifts", true)
	backupMatches, err := filepath.Glob(databasePath + ".backup-*")
	if err != nil || len(backupMatches) != 1 {
		t.Fatalf("expected one database backup, found %v (err=%v)", backupMatches, err)
	}
}

func TestInvoicePaymentEntriesMigrationRepairsMissingTable(t *testing.T) {
	conn := openTestDatabase(t)
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.version > 20 {
			break
		}
		if _, err := conn.Exec(migration.sql); err != nil {
			t.Fatalf("apply migration %d: %v", migration.version, err)
		}
	}
	if _, err := conn.Exec(`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.version > 20 {
			break
		}
		if _, err := conn.Exec("INSERT INTO schema_migrations(version, name) VALUES (?, ?)", migration.version, migration.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec("DROP TABLE invoice_payment_entries"); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("repair missing payment-entry table: %v", err)
	}
	assertMigrationVersion(t, conn, 65)
	assertTableExists(t, conn, "invoice_payment_entries")
	assertIndexExists(t, conn, "idx_clients_phone", "clients")
	assertIndexExists(t, conn, "idx_products_barcode", "products")
}

func TestBranchInvoiceNumberingMigrationPreservesHighestYearlyIndex(t *testing.T) {
	conn := openTestDatabase(t)
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var numberingMigration migration
	for _, migration := range migrations {
		if migration.version < 20 {
			if _, err := conn.Exec(migration.sql); err != nil {
				t.Fatalf("apply migration %d: %v", migration.version, err)
			}
			continue
		}
		if migration.version == 20 {
			numberingMigration = migration
			break
		}
	}
	if _, err := conn.Exec(`INSERT INTO invoice_indexes(year, cashbox_id, last_index, type)
		VALUES (2026, 1, 4, 'sales'), (2026, 2, 7, 'sales'), (2026, 1, 3, 'return')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(numberingMigration.sql); err != nil {
		t.Fatal(err)
	}

	var salesIndex, exchangeIndex int64
	if err := conn.QueryRow(`SELECT last_index FROM invoice_indexes WHERE year = 2026 AND type = 'sales'`).Scan(&salesIndex); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`SELECT last_index FROM invoice_indexes WHERE year = 2026 AND type = 'return'`).Scan(&exchangeIndex); err != nil {
		t.Fatal(err)
	}
	if salesIndex != 7 || exchangeIndex != 3 {
		t.Fatalf("migrated indexes: sales=%d exchange=%d", salesIndex, exchangeIndex)
	}
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func assertMigrationVersion(t *testing.T, conn *sql.DB, expected int) {
	t.Helper()
	var actual int
	if err := conn.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("migration version = %d, want %d", actual, expected)
	}
}

func assertTableExists(t *testing.T, conn *sql.DB, table string) {
	t.Helper()
	var count int
	if err := conn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("table %q does not exist", table)
	}
}

func assertColumnExists(t *testing.T, conn *sql.DB, table, column string) {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	if err := conn.QueryRow(query, column).Scan(&count); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if count != 1 {
		t.Fatalf("column %s.%s does not exist", table, column)
	}
}

func assertIndexExists(t *testing.T, conn *sql.DB, index, table string) {
	t.Helper()
	var count int
	if err := conn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ? AND tbl_name = ?", index, table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index %q on table %q does not exist", index, table)
	}
}

func assertFeatureFlagEnabled(t *testing.T, conn *sql.DB, key string, want bool) {
	t.Helper()
	var enabled int
	if err := conn.QueryRow("SELECT enabled FROM feature_flags WHERE key = ?", key).Scan(&enabled); err != nil {
		t.Fatalf("feature flag %q: %v", key, err)
	}
	if (enabled != 0) != want {
		t.Fatalf("feature flag %q enabled = %d, want %v", key, enabled, want)
	}
}

func TestLookupIndexesAreRepairedForCurrentDatabase(t *testing.T) {
	conn := openTestDatabase(t)
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := conn.Exec("DROP INDEX idx_clients_phone; DROP INDEX idx_products_barcode"); err != nil {
		t.Fatalf("drop lookup indexes: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 26"); err != nil {
		t.Fatalf("rewind migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 27"); err != nil {
		t.Fatalf("rewind variant migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 28"); err != nil {
		t.Fatalf("rewind attendance migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 29"); err != nil {
		t.Fatalf("rewind custom-discount migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 30"); err != nil {
		t.Fatalf("rewind shift-closing migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 31"); err != nil {
		t.Fatalf("rewind per-unit pricing migration metadata: %v", err)
	}
	if _, err := conn.Exec("DELETE FROM schema_migrations WHERE version = 32"); err != nil {
		t.Fatalf("rewind per-unit discount migration metadata: %v", err)
	}
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("repair lookup indexes: %v", err)
	}
	assertIndexExists(t, conn, "idx_clients_phone", "clients")
	assertIndexExists(t, conn, "idx_products_barcode", "products")
}

// TestSelfHealsColumnsMissingDespiteRecordedMigrationVersion guards against
// the exact bug reported in production: schema_migrations recorded versions
// 52 (currency_rates), 54 (shift_expected_counts), and 57 (line salesperson)
// as applied - because an
// earlier draft of that migration's SQL ran against this database before
// the migration content was finalized, and migrations are tracked by
// version number only, not by content hash - while the columns those
// migrations are supposed to add were never actually present. Without a
// self-heal, every currency lookup fails with "no such column:
// units_per_usd_micros" forever, since applyMigrations sees current >=
// latest and never retries version 52's SQL.
func TestSelfHealsColumnsMissingDespiteRecordedMigrationVersion(t *testing.T) {
	conn := openTestDatabase(t)
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := conn.Exec("ALTER TABLE currencies DROP COLUMN units_per_usd_micros"); err != nil {
		t.Fatalf("simulate missing currency column: %v", err)
	}
	if _, err := conn.Exec("ALTER TABLE currencies DROP COLUMN cash_rounding_unit"); err != nil {
		t.Fatalf("simulate missing currency column: %v", err)
	}
	if _, err := conn.Exec("ALTER TABLE shifts DROP COLUMN expected_usd"); err != nil {
		t.Fatalf("simulate missing shift column: %v", err)
	}
	if _, err := conn.Exec("ALTER TABLE invoice_products DROP COLUMN salesperson_id"); err != nil {
		t.Fatalf("simulate missing invoice product salesperson column: %v", err)
	}
	// schema_migrations is untouched - versions 52/54 are still recorded as
	// applied, exactly as they'd be if only their SQL was ever different.

	if err := applyMigrations(context.Background(), conn, "", true); err != nil {
		t.Fatalf("self-heal migration: %v", err)
	}

	var unitsPerUsdMicros int64
	if err := conn.QueryRow("SELECT units_per_usd_micros FROM currencies WHERE code = 'LBP'").Scan(&unitsPerUsdMicros); err != nil {
		t.Fatalf("currencies.units_per_usd_micros not restored: %v", err)
	}
	if unitsPerUsdMicros <= 0 {
		t.Fatalf("currencies.units_per_usd_micros = %d, want > 0", unitsPerUsdMicros)
	}
	var expectedUsd int64
	if err := conn.QueryRow("SELECT expected_usd FROM shifts LIMIT 1").Scan(&expectedUsd); err != nil && err != sql.ErrNoRows {
		t.Fatalf("shifts.expected_usd not restored: %v", err)
	}
	assertColumnExists(t, conn, "invoice_products", "salesperson_id")
}

func TestApplicationDatabasePathOverride(t *testing.T) {
	configured := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv("KASHI_DB_PATH", configured)
	path, err := applicationDatabasePath()
	if err != nil {
		t.Fatal(err)
	}
	if path != configured {
		t.Fatalf("database path = %q, want %q", path, configured)
	}
}

func TestStartupPreservesRuntimeDatabase(t *testing.T) {
	t.Setenv("KASHI_DB_PATH", filepath.Join(t.TempDir(), "database"))
	first := NewApp()
	first.startup(context.Background())
	if err := first.SetTheme("dark"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	first.shutdown(context.Background())

	second := NewApp()
	second.startup(context.Background())
	defer second.shutdown(context.Background())
	if startupError := second.GetStartupError(); startupError != "" {
		t.Fatalf("second startup failed: %s", startupError)
	}
	theme, err := second.GetTheme()
	if err != nil {
		t.Fatalf("get theme after restart: %v", err)
	}
	if theme != "dark" {
		t.Fatalf("theme after restart = %q, want dark", theme)
	}
}
