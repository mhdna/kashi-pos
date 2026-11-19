package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Encryption at rest. With licensing on, every branch database on the till is
// encrypted with the install's database key (from the license server, sealed
// to this PC in license.dat), through ncruces's "adiantum" VFS. A database
// copied off the till opens nowhere else: not in another kashi-pos, not in a
// SQLite tool. The vendor opens one with cmd/kashi-db-unlock and the key from
// the license server.
//
// An existing plain database is converted the first time it is opened with a
// key (convertToEncrypted). Without licensing (wails dev, tests) databases
// stay plain.

const encTmpSuffix = ".enc-tmp"

// sqliteHeader starts every plain SQLite file; an encrypted one doesn't.
var sqliteHeader = []byte("SQLite format 3\x00")

// databaseKey is the key the app's databases open with, or "" for plain.
func (a *App) databaseKey() string {
	if !licenseRequired || a.license == nil {
		return ""
	}
	return a.license.DBKey()
}

// encryptedDatabaseDSN is databaseDSN for an encrypted file. The VFS and key
// come first; mmap is left out because the VFS decrypts pages as it reads
// them, which a memory map would bypass.
func encryptedDatabaseDSN(path, keyHex string) string {
	var keep []string
	for _, p := range strings.Split(strings.SplitN(databaseDSN(path), "?", 2)[1], "&") {
		if !strings.HasPrefix(p, "_pragma=mmap_size") {
			keep = append(keep, p)
		}
	}
	return "file:" + uriPath(path) + "?vfs=adiantum&hexkey=" + keyHex + "&" + strings.Join(keep, "&")
}

// openAppDatabase opens path with the key, converting a plain file first.
// A failed conversion leaves the plain file exactly as it was and opens it
// plain, so the till keeps working; the conversion is retried next launch.
func openAppDatabase(path, keyHex string) (*sql.DB, error) {
	if keyHex == "" {
		return sql.Open("sqlite", databaseDSN(path))
	}
	_ = os.Remove(path + encTmpSuffix)
	plain, err := isPlainSQLite(path)
	if err != nil {
		return nil, err
	}
	if plain {
		if err := convertToEncrypted(path, keyHex); err != nil {
			log.Printf("encrypt database (kept it plain, will retry next launch): %v", err)
			return sql.Open("sqlite", databaseDSN(path))
		}
	}
	return sql.Open("sqlite", encryptedDatabaseDSN(path, keyHex))
}

// isPlainSQLite reports whether path is an unencrypted SQLite file. A missing
// or empty file is not.
func isPlainSQLite(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(f, head); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(head, sqliteHeader), nil
}

// convertToEncrypted rewrites a plain database as an encrypted one without
// risking it:
//
//  1. fold the WAL into the main file, so the copy sees every committed row;
//  2. back the file up into an encrypted temp file beside it;
//  3. check the copy: integrity_check, and the same row count in every table;
//  4. only then rename the copy over the original (atomic on one filesystem).
//
// Until step 4 the original is untouched; after it, the file is the checked
// copy. Never half of either.
func convertToEncrypted(path, keyHex string) error {
	tmp := path + encTmpSuffix
	src, err := sqlite3.Open("file:" + uriPath(path))
	if err != nil {
		return fmt.Errorf("open plain database: %w", err)
	}
	if err := src.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		src.Close()
		return fmt.Errorf("checkpoint: %w", err)
	}
	if err := src.Backup("main", "file:"+uriPath(tmp)+"?vfs=adiantum&hexkey="+keyHex); err != nil {
		src.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write encrypted copy: %w", err)
	}
	if err := src.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := verifyEncryptedCopy(path, tmp, keyHex); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("check encrypted copy: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace database: %w", err)
	}
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	log.Printf("database encrypted at rest: %s", path)
	return nil
}

func verifyEncryptedCopy(plainPath, encPath, keyHex string) error {
	enc, err := sqlite3.Open("file:" + uriPath(encPath) + "?vfs=adiantum&hexkey=" + keyHex + "&mode=ro")
	if err != nil {
		return err
	}
	defer enc.Close()
	var integrity string
	if err := scanOneRaw(enc, `PRAGMA integrity_check`, &integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity_check: %s", integrity)
	}
	src, err := sqlite3.Open("file:" + uriPath(plainPath) + "?mode=ro")
	if err != nil {
		return err
	}
	defer src.Close()
	want, err := tableRowCounts(src)
	if err != nil {
		return err
	}
	got, err := tableRowCounts(enc)
	if err != nil {
		return err
	}
	if len(want) != len(got) {
		return fmt.Errorf("%d tables plain, %d encrypted", len(want), len(got))
	}
	for name, n := range want {
		if got[name] != n {
			return fmt.Errorf("table %s: %d rows plain, %d encrypted", name, n, got[name])
		}
	}
	return nil
}

func tableRowCounts(conn *sqlite3.Conn) (map[string]int64, error) {
	stmt, _, err := conn.Prepare(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	var names []string
	for stmt.Step() {
		names = append(names, stmt.ColumnText(0))
	}
	err = stmt.Err()
	stmt.Close()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(names))
	for _, name := range names {
		var n int64
		// Virtual tables (FTS5) are counted too; their shadow tables are
		// ordinary tables and listed on their own.
		if err := scanOneRaw(conn, `SELECT COUNT(*) FROM "`+strings.ReplaceAll(name, `"`, `""`)+`"`, &n); err != nil {
			return nil, fmt.Errorf("count %s: %w", name, err)
		}
		counts[name] = n
	}
	return counts, nil
}

func scanOneRaw(conn *sqlite3.Conn, query string, dst any) error {
	stmt, _, err := conn.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if !stmt.Step() {
		if err := stmt.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	switch d := dst.(type) {
	case *string:
		*d = stmt.ColumnText(0)
	case *int64:
		*d = stmt.ColumnInt64(0)
	default:
		return fmt.Errorf("scanOneRaw: unsupported %T", dst)
	}
	return nil
}

// vacuumTarget is what to pass to VACUUM INTO for a copy at path. On an
// encrypted database the copy must be opened through the same VFS with the
// same key (a plain path fails with "unable to open database file"), so a
// backup is encrypted too.
func (a *App) vacuumTarget(path string) string {
	if key := a.databaseKey(); key != "" {
		return "file:" + uriPath(path) + "?vfs=adiantum&hexkey=" + key
	}
	return path
}
