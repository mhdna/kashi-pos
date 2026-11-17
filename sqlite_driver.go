package main

import (
	"database/sql"
	"database/sql/driver"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/ncruces/go-sqlite3"
	ncdriver "github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"
)

// The app's SQLite driver is ncruces/go-sqlite3 (SQLite compiled to
// WebAssembly, no cgo), chosen over modernc.org/sqlite because it can encrypt
// the database file: the "adiantum" VFS (see database_encryption.go).
//
// It is registered as "sqlite", the name modernc used, and it accepts
// modernc's connection strings ("C:\dir\database?_pragma=foreign_keys(ON)"),
// so the many sql.Open("sqlite", ...) calls keep working. ncruces itself only
// reads "?..." options from a "file:" URI; given a plain path it would treat
// "?_pragma=..." as part of the file name and silently drop every pragma.
// sqliteURI does that translation.
//
// FTS5 (product search, migration 135) is a separate extension in ncruces;
// AutoExtension loads it on every connection, as modernc had it built in.
func init() {
	sqlite3.AutoExtension(fts5.Register)
	sql.Register("sqlite", sqliteDriver{})
}

type sqliteDriver struct{}

var nc = &ncdriver.SQLite{}

func (sqliteDriver) Open(name string) (driver.Conn, error) { return nc.Open(sqliteURI(name)) }

func (sqliteDriver) OpenConnector(name string) (driver.Connector, error) {
	return nc.OpenConnector(sqliteURI(name))
}

// sqliteURI turns "path?query" into "file:/path?query". A name that is
// already a "file:" URI, or ":memory:", passes through. Two defaults keep
// modernc's behaviour where the caller didn't choose:
//
//   - foreign keys off (ncruces's build turns them on; modernc left SQLite's
//     default, off). The app's own connection always asks for ON.
//   - time values bind in SQLite's text format ("2006-01-02 15:04:05.000+00:00"),
//     closest to what modernc wrote, not RFC 3339.
func sqliteURI(name string) string {
	if strings.HasPrefix(name, "file:") || name == ":memory:" {
		return name
	}
	path, query, _ := strings.Cut(name, "?")
	add := func(param string) {
		if query != "" {
			query += "&"
		}
		query += param
	}
	if !strings.Contains(query, "foreign_keys") {
		add("_pragma=foreign_keys(OFF)")
	}
	if !strings.Contains(query, "_timefmt=") {
		add("_timefmt=sqlite")
	}
	return "file:" + uriPath(path) + "?" + query
}

// uriPath turns a filesystem path into the path part of a "file:" URI:
// forward slashes (so Windows "C:\dir\db" becomes "/C:/dir/db", which SQLite
// maps back to the drive), a leading slash, and percent-encoding of spaces
// and other characters that mean something in a URI.
func uriPath(p string) string {
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		if abs, err := filepath.Abs(p); err == nil {
			s = filepath.ToSlash(abs)
		}
		if !strings.HasPrefix(s, "/") {
			s = "/" + s
		}
	}
	return (&url.URL{Path: s}).EscapedPath()
}
