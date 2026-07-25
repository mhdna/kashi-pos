package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyMigrationsFreshDatabase(t *testing.T) {
	conn := openTestDatabase(t)
	if err := applyMigrations(context.Background(), conn, "", false); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	assertMigrationVersion(t, conn, 19)
	assertTableExists(t, conn, "settings")
	assertTableExists(t, conn, "schema_migrations")
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
	assertMigrationVersion(t, conn, 19)
	assertTableExists(t, conn, "schema_migrations")
	backupMatches, err := filepath.Glob(databasePath + ".backup-*")
	if err != nil || len(backupMatches) != 1 {
		t.Fatalf("expected one database backup, found %v (err=%v)", backupMatches, err)
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
