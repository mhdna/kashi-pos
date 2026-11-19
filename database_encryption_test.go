package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDBKey = "8f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

// plainTillDatabase makes a migrated, plain till database with some rows.
func plainTillDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database")
	if err := os.WriteFile(path, embeddedDB, 0o600); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", databaseDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(context.Background(), conn, path, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		if _, err := conn.Exec(`INSERT INTO clients (name, phone) VALUES (?, ?)`, "Client "+strings.Repeat("x", i%7), fmt.Sprintf("03%06d", i)); err != nil {
			t.Fatal(err)
		}
	}
	conn.Close()
	return path
}

func countRows(t *testing.T, conn *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPlainDatabaseIsEncryptedOnFirstOpenWithKey(t *testing.T) {
	path := plainTillDatabase(t)
	before, _ := sql.Open("sqlite", databaseDSN(path))
	clients, products := countRows(t, before, "clients"), countRows(t, before, "products")
	before.Close()

	conn, err := openAppDatabase(path, testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, conn, "clients"); got != clients || got < 300 {
		t.Fatalf("clients %d after encryption, %d before", got, clients)
	}
	if got := countRows(t, conn, "products"); got != products {
		t.Fatalf("products %d, %d before", got, products)
	}
	// Writes and the FTS index keep working on the encrypted file.
	if _, err := conn.Exec(`INSERT INTO clients (name, phone) VALUES ('After', '0311')`); err != nil {
		t.Fatal(err)
	}
	var integrity string
	if err := conn.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity %q %v", integrity, err)
	}
	conn.Close()

	if plain, _ := isPlainSQLite(path); plain {
		t.Fatal("file is still plain SQLite")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "Client x") || strings.Contains(string(raw), "CREATE TABLE") {
		t.Fatal("database text readable in the file")
	}
	for _, leftover := range []string{encTmpSuffix, "-wal"} {
		if _, err := os.Stat(path + leftover); err == nil && leftover == encTmpSuffix {
			t.Fatalf("%s left behind", leftover)
		}
	}

	// Without the key, or with another one, nothing opens.
	for name, dsn := range map[string]string{
		"no key":    databaseDSN(path),
		"wrong key": encryptedDatabaseDSN(path, strings.Repeat("ab", 32)),
	} {
		other, _ := sql.Open("sqlite", dsn)
		var n int
		if err := other.QueryRow(`SELECT COUNT(*) FROM clients`).Scan(&n); err == nil {
			t.Fatalf("%s: read %d clients from the encrypted file", name, n)
		}
		other.Close()
	}

	// Opening again doesn't convert again, and sees the new row.
	again, err := openAppDatabase(path, testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := countRows(t, again, "clients"); got != clients+1 {
		t.Fatalf("reopen: %d clients, want %d", got, clients+1)
	}
}

func TestNewDatabaseFromTemplateIsEncrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database")
	_ = os.WriteFile(path, embeddedDB, 0o600)
	conn, err := openAppDatabase(path, testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := applyMigrations(context.Background(), conn, path, false); err != nil {
		t.Fatalf("migrations on an encrypted database: %v", err)
	}
	if plain, _ := isPlainSQLite(path); plain {
		t.Fatal("new database is plain")
	}
}

func TestFailedConversionKeepsThePlainDatabaseWorking(t *testing.T) {
	path := plainTillDatabase(t)
	// A directory where the temp copy goes makes the copy fail.
	if err := os.Mkdir(path+encTmpSuffix+"-blocker", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := convertToEncrypted(path, "not-hex"); err == nil {
		t.Fatal("conversion with a bad key succeeded")
	}
	if plain, _ := isPlainSQLite(path); !plain {
		t.Fatal("a failed conversion touched the original")
	}
	conn, err := openAppDatabase(path, "zz") // bad key: conversion fails, opens plain
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if countRows(t, conn, "clients") < 300 {
		t.Fatal("plain database lost rows")
	}
	if _, err := os.Stat(path + encTmpSuffix); err == nil {
		t.Fatal("temp copy left behind")
	}
}

func TestPendingWALRowsSurviveConversion(t *testing.T) {
	path := plainTillDatabase(t)
	// A writer that leaves its rows in the WAL (no checkpoint at close).
	conn, _ := sql.Open("sqlite", databaseDSN(path))
	conn.SetMaxOpenConns(1)
	_, _ = conn.Exec(`PRAGMA wal_autocheckpoint=0`)
	if _, err := conn.Exec(`INSERT INTO clients (name, phone) VALUES ('InWal', '0399')`); err != nil {
		t.Fatal(err)
	}
	want := countRows(t, conn, "clients")
	// Copy the files as they are mid-run, as a crash would leave them.
	crash := filepath.Join(t.TempDir(), "database")
	for _, suffix := range []string{"", "-wal"} {
		b, err := os.ReadFile(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(crash+suffix, b, 0o600)
	}
	conn.Close()
	enc, err := openAppDatabase(crash, testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	if got := countRows(t, enc, "clients"); got != want {
		t.Fatalf("%d clients after conversion, want %d (WAL rows lost)", got, want)
	}
}

func TestVacuumIntoFromEncryptedDatabaseIsEncrypted(t *testing.T) {
	path := plainTillDatabase(t)
	conn, err := openAppDatabase(path, testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	out := filepath.Join(t.TempDir(), "backup.db")
	// A plain path can't be written through the encrypting VFS...
	if _, err := conn.Exec(`VACUUM INTO ?`, out); err == nil {
		t.Fatal("VACUUM INTO a plain path worked; vacuumTarget may not be needed")
	}
	_ = os.Remove(out)
	// ...the keyed URI from vacuumTarget can.
	target := "file:" + uriPath(out) + "?vfs=adiantum&hexkey=" + testDBKey
	if _, err := conn.Exec(`VACUUM INTO ?`, target); err != nil {
		t.Fatal(err)
	}
	if plain, _ := isPlainSQLite(out); plain {
		t.Fatal("VACUUM INTO wrote a plain copy of an encrypted database")
	}
	back, _ := sql.Open("sqlite", encryptedDatabaseDSN(out, testDBKey))
	defer back.Close()
	if countRows(t, back, "clients") < 300 {
		t.Fatal("backup does not open with the key")
	}
}

func TestEncryptedDSNDropsMmapKeepsTheRest(t *testing.T) {
	dsn := encryptedDatabaseDSN("/x/database", testDBKey)
	if !strings.HasPrefix(dsn, "file:/x/database?vfs=adiantum&hexkey="+testDBKey+"&") {
		t.Fatal(dsn)
	}
	if strings.Contains(dsn, "mmap") {
		t.Fatal("mmap on an encrypted database")
	}
	for _, p := range []string{"journal_mode(WAL)", "foreign_keys(ON)", "busy_timeout(5000)", "_txlock=immediate", "synchronous(NORMAL)"} {
		if !strings.Contains(dsn, p) {
			t.Fatalf("missing %s in %s", p, dsn)
		}
	}
}
