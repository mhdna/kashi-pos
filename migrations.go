package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// migrationFiles is kept next to the reviewed SQL files. Runtime databases are
// never replaced with this seed; these files are only used to upgrade them.
//
//go:embed db/migrations/*.up.sql
var migrationFiles embed.FS

const migrationTable = "schema_migrations"

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.Glob(migrationFiles, "db/migrations/*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)

	migrations := make([]migration, 0, len(entries))
	for _, name := range entries {
		base := filepath.Base(name)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", base)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", base)
		}
		contents, err := fs.ReadFile(migrationFiles, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", base, err)
		}
		migrations = append(migrations, migration{version: version, name: base, sql: string(contents)})
	}
	return migrations, nil
}

func applyMigrations(ctx context.Context, conn *sql.DB, databasePath string, databaseExisted bool) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if databaseExisted {
		needsUpgrade, err := schemaNeedsUpgrade(ctx, conn, migrations)
		if err != nil {
			return err
		}
		if needsUpgrade {
			if err := backupDatabase(databasePath); err != nil {
				return err
			}
		}
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema migration table: %w", err)
	}

	if err := bootstrapMigrationMetadata(ctx, conn, migrations); err != nil {
		return err
	}
	current, err := currentMigrationVersion(ctx, conn)
	if err != nil {
		return err
	}
	if current >= latestMigrationVersion(migrations) {
		return ensureReceiptBrandingColumns(ctx, conn)
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if m.version == 18 {
			needsRepair, err := invoiceIndexNeedsRepair(ctx, conn)
			if err != nil {
				return err
			}
			if !needsRepair {
				if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES (?, ?)", m.version, m.name); err != nil {
					return fmt.Errorf("record already-satisfied migration %d: %w", m.version, err)
				}
				log.Printf("database migration already satisfied: version=%d name=%s", m.version, m.name)
				continue
			}
		}
		if err := applyOneMigration(ctx, conn, m); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
		log.Printf("database migration applied: version=%d name=%s", m.version, m.name)
	}
	if err := ensureReceiptBrandingColumns(ctx, conn); err != nil {
		return err
	}
	return nil
}

func ensureReceiptBrandingColumns(ctx context.Context, conn *sql.DB) error {
	columns := []struct{ name, definition string }{
		{"market_name", "text NOT NULL DEFAULT ''"}, {"market_phone", "text NOT NULL DEFAULT ''"},
		{"market_description", "text NOT NULL DEFAULT 'Shop for all'"}, {"return_policy", "text NOT NULL DEFAULT 'Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt.'"},
		{"website", "text NOT NULL DEFAULT 'yourwebsite.com'"}, {"instagram", "text NOT NULL DEFAULT '@your_instagram'"}, {"receipt_font", "text NOT NULL DEFAULT 'Belleza'"},
		{"social_platforms", "text NOT NULL DEFAULT '[]'"}, {"social_handles", "text NOT NULL DEFAULT '{}'"},
		{"receipt_title_size", "real NOT NULL DEFAULT 21"}, {"receipt_body_size", "real NOT NULL DEFAULT 11"}, {"receipt_cutoff", "integer NOT NULL DEFAULT 1"},
		{"receipt_body_font", "text NOT NULL DEFAULT 'Printer Font A'"}, {"branch_phone", "text NOT NULL DEFAULT ''"}, {"counter_number", "text NOT NULL DEFAULT '1'"},
	}
	for _, column := range columns {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('settings') WHERE name = ?", column.name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := conn.ExecContext(ctx, "ALTER TABLE settings ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaNeedsUpgrade(ctx context.Context, conn *sql.DB, migrations []migration) (bool, error) {
	var migrationTableExists int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'").Scan(&migrationTableExists); err != nil {
		return false, fmt.Errorf("inspect migration metadata: %w", err)
	}
	if migrationTableExists != 0 {
		current, err := currentMigrationVersion(ctx, conn)
		if err != nil {
			return false, err
		}
		return current < latestMigrationVersion(migrations), nil
	}
	var hasRoles int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'roles'").Scan(&hasRoles); err != nil {
		return false, fmt.Errorf("inspect existing schema: %w", err)
	}
	if hasRoles == 0 {
		return false, nil
	}
	var hasTaxRate int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('settings') WHERE name = 'tax_rate'").Scan(&hasTaxRate); err != nil {
		return false, fmt.Errorf("inspect settings schema: %w", err)
	}
	if hasTaxRate == 0 {
		return true, nil
	}
	return invoiceIndexNeedsRepair(ctx, conn)
}

// Older releases did not record versions. Existing databases correspond to
// the schema represented by migrations 1-16, plus optional later fixes.
func bootstrapMigrationMetadata(ctx context.Context, conn *sql.DB, migrations []migration) error {
	var count int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		return fmt.Errorf("inspect schema migration table: %w", err)
	}
	if count != 0 {
		return nil
	}

	var hasRoles int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'roles'").Scan(&hasRoles); err != nil {
		return fmt.Errorf("inspect existing schema: %w", err)
	}
	if hasRoles == 0 {
		return nil // genuinely empty database; run all migrations.
	}

	baseline := 16
	var hasTaxRate int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('settings') WHERE name = 'tax_rate'").Scan(&hasTaxRate); err != nil {
		return fmt.Errorf("inspect settings schema: %w", err)
	}
	if hasTaxRate != 0 {
		baseline = 17
	}
	indexNeedsRepair, err := invoiceIndexNeedsRepair(ctx, conn)
	if err != nil {
		return err
	}
	if !indexNeedsRepair {
		baseline = 18
	}
	maxVersion := latestMigrationVersion(migrations)
	if baseline > maxVersion {
		baseline = maxVersion
	}
	for _, m := range migrations {
		if m.version > baseline {
			break
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES (?, ?)", m.version, m.name); err != nil {
			return fmt.Errorf("record baseline migration %d: %w", m.version, err)
		}
	}
	log.Printf("database migration baseline recorded: version=%d", baseline)
	return nil
}

func invoiceIndexNeedsRepair(ctx context.Context, conn *sql.DB) (bool, error) {
	var indexSQL sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'invoice_indexes'").Scan(&indexSQL); err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("inspect invoice index schema: %w", err)
	}
	return !strings.Contains(strings.ToLower(indexSQL.String), "primary key (cashbox_id, year, type)"), nil
}

func currentMigrationVersion(ctx context.Context, conn *sql.DB) (int, error) {
	var version sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema migration version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

func latestMigrationVersion(migrations []migration) int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}

func applyOneMigration(ctx context.Context, conn *sql.DB, m migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES (?, ?)", m.version, m.name); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit()
}

func backupDatabase(databasePath string) error {
	backupPath := fmt.Sprintf("%s.backup-%s", databasePath, time.Now().UTC().Format("20060102-150405"))
	if err := copyFile(databasePath, backupPath); err != nil {
		return fmt.Errorf("backup database before migration: %w", err)
	}
	log.Printf("database backup created: %s", backupPath)
	return nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
