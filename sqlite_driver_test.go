package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSQLiteURI(t *testing.T) {
	abs := func(p string) string { a, _ := filepath.Abs(p); return filepath.ToSlash(a) }
	cases := map[string]string{
		"/data/kashi/database":                          "file:/data/kashi/database?_pragma=foreign_keys(OFF)&_timefmt=sqlite",
		"/data/kashi/database?_pragma=foreign_keys(ON)": "file:/data/kashi/database?_pragma=foreign_keys(ON)&_timefmt=sqlite",
		"/data/my shop/db#1?_txlock=immediate":          "file:/data/my%20shop/db%231?_txlock=immediate&_pragma=foreign_keys(OFF)&_timefmt=sqlite",
		"/x?_pragma=foreign_keys(1)&_timefmt=rfc3339":   "file:/x?_pragma=foreign_keys(1)&_timefmt=rfc3339",
		"file:/already?vfs=adiantum":                    "file:/already?vfs=adiantum",
		":memory:":                                      ":memory:",
		"rel/db":                                        "file:" + abs("rel/db") + "?_pragma=foreign_keys(OFF)&_timefmt=sqlite",
	}
	for in, want := range cases {
		if got := sqliteURI(in); got != want {
			t.Errorf("sqliteURI(%q)\n got %q\nwant %q", in, got, want)
		}
	}
	if runtime.GOOS == "windows" {
		if got := sqliteURI(`C:\ProgramData\kashi\database`); got != "file:/C:/ProgramData/kashi/database?_pragma=foreign_keys(OFF)&_timefmt=sqlite" {
			t.Errorf("windows path = %q", got)
		}
	}
}

// The options in a modernc-style "path?query" string must reach SQLite
// (ncruces alone would make them part of the file name), and the file must
// be created at the path, not at "path?query".
func TestSQLiteDriverAppliesPragmasFromPlainPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my db")
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var fk int
	var mode string
	if err := conn.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if fk != 1 || mode != "wal" {
		t.Fatalf("foreign_keys=%d journal_mode=%s", fk, mode)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not at its path: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "?") || strings.Contains(e.Name(), "pragma") {
			t.Fatalf("stray file %q: options became part of the name", e.Name())
		}
	}
	// No option given: foreign keys stay off, as they were under modernc.
	plain, _ := sql.Open("sqlite", filepath.Join(dir, "plain"))
	defer plain.Close()
	if err := plain.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 0 {
		t.Fatalf("default foreign_keys=%d err=%v", fk, err)
	}
	// FTS5 is loaded on every connection (product search needs it).
	if _, err := plain.Exec(`CREATE VIRTUAL TABLE f USING fts5(x)`); err != nil {
		t.Fatalf("fts5: %v", err)
	}
}

func TestIsDatabaseBusyOnARealLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.db")
	holder, _ := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)")
	defer holder.Close()
	if _, err := holder.Exec(`CREATE TABLE t(x)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := holder.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	other, _ := sql.Open("sqlite", path+"?_pragma=busy_timeout(0)")
	defer other.Close()
	start := time.Now()
	_, err = other.Exec(`INSERT INTO t VALUES (1)`)
	if err == nil {
		t.Fatal("write went through a held write lock")
	}
	if !isDatabaseBusy(err) {
		t.Fatalf("isDatabaseBusy(%v) = false", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("busy_timeout(0) was not applied")
	}
	if isDatabaseBusy(sql.ErrNoRows) {
		t.Fatal("ErrNoRows counted as busy")
	}
	_, _ = c.ExecContext(ctx, `ROLLBACK`)
}
