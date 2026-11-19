package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"
)

const key = "8f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

func TestUnlockWritesAReadablePlainCopy(t *testing.T) {
	dir := t.TempDir()
	enc := filepath.Join(dir, "database")
	c, err := sqlite3.Open("file:" + uri(enc) + "?vfs=adiantum&hexkey=" + key)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Exec(`CREATE TABLE invoices(code TEXT); INSERT INTO invoices VALUES ('INV-1'), ('INV-2')`); err != nil {
		t.Fatal(err)
	}
	c.Close()

	if err := Unlock(enc, strings.Repeat("00", 32), filepath.Join(dir, "bad.db")); err == nil {
		t.Fatal("wrong key unlocked the database")
	}
	out := filepath.Join(dir, "plain.db")
	if err := Unlock(enc, key, out); err != nil {
		t.Fatal(err)
	}
	p, err := sqlite3.Open("file:" + uri(out))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	stmt, _, err := p.Prepare(`SELECT COUNT(*) FROM invoices`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if !stmt.Step() || stmt.ColumnInt(0) != 2 {
		t.Fatal("plain copy is missing rows")
	}
	if err := run(enc, key, "", "", "", out); err == nil {
		t.Fatal("overwrote an existing output")
	}
}
