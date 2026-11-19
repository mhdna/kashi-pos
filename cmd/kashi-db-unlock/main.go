// kashi-db-unlock writes a plain copy of a till's encrypted database, so the
// vendor can open it on another machine (sqlite3, DB Browser, a dev till).
//
// The key is the till's database key from the license server: shown on the
// customer's page under the till's install, or fetched here with the admin
// token.
//
//	go run ./cmd/kashi-db-unlock -db ~/Downloads/database -key <64 hex> -out plain.db
//	go run ./cmd/kashi-db-unlock -db database -install <install id> -server https://license.grandbz.com -token $ADMIN_TOKEN
//
// The plain copy is readable by anyone who has the file: keep it on your
// machine and delete it when done.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/ext/fts5"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"
)

func main() {
	dbPath := flag.String("db", "", "the till's encrypted database file")
	key := flag.String("key", "", "database key (64 hex characters)")
	install := flag.String("install", "", "install id, to fetch the key from the license server")
	server := flag.String("server", "https://license.grandbz.com", "license server")
	token := flag.String("token", os.Getenv("KASHI_LICENSE_ADMIN_TOKEN"), "license server ADMIN_TOKEN")
	out := flag.String("out", "", "plain copy to write (default: <db>.plain.db)")
	flag.Parse()
	if err := run(*dbPath, *key, *install, *server, *token, *out); err != nil {
		fmt.Fprintln(os.Stderr, "kashi-db-unlock:", err)
		os.Exit(1)
	}
}

func run(dbPath, key, install, server, token, out string) error {
	if dbPath == "" {
		return errors.New("-db is required")
	}
	if key == "" && install != "" {
		k, err := fetchKey(server, install, token)
		if err != nil {
			return err
		}
		key = k
	}
	if len(key) != 64 {
		return errors.New("need -key (64 hex characters) or -install with -token")
	}
	if out == "" {
		out = dbPath + ".plain.db"
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists", out)
	}
	if err := Unlock(dbPath, key, out); err != nil {
		return err
	}
	fmt.Println("wrote", out)
	return nil
}

func uri(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	s := filepath.ToSlash(abs)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return (&url.URL{Path: s}).EscapedPath()
}

// Unlock copies the encrypted database at src into a plain file at dst.
func Unlock(src, keyHex, dst string) error {
	sqlite3.AutoExtension(fts5.Register)
	conn, err := sqlite3.Open("file:" + uri(src) + "?vfs=adiantum&hexkey=" + keyHex + "&mode=ro")
	if err != nil {
		return err
	}
	defer conn.Close()
	stmt, _, err := conn.Prepare(`PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("wrong key, or not an encrypted till database: %w", err)
	}
	ok := stmt.Step() && stmt.ColumnText(0) == "ok"
	stmt.Close()
	if !ok {
		return errors.New("wrong key, or not an encrypted till database")
	}
	return conn.Backup("main", "file:"+uri(dst))
}

func fetchKey(server, install, token string) (string, error) {
	if token == "" {
		return "", errors.New("-token (or KASHI_LICENSE_ADMIN_TOKEN) is required with -install")
	}
	req, err := http.NewRequest("GET", strings.TrimRight(server, "/")+"/admin/api/installs/"+url.PathEscape(install), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		DBKey string `json:"dbKey"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("license server: %d %s", resp.StatusCode, body.Error)
	}
	return body.DBKey, nil
}
