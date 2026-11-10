package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// photoCache keeps this till's copies of kashi's photos (expense, loan and
// transfer photos, their thumbnails, and the lists of which photos a record
// has) so a page shows them straight from disk instead of waiting on kashi.
// record_photos.go decides when to use and refresh a copy; this file only
// stores them.
//
// The copies are not meant to be browsable. There are no image files: every
// copy is a row in one SQLite file in a hidden folder under the OS cache
// directory, and the bytes are AES-256-GCM encrypted. The key is
// photoCacheSecret (compiled into the app) mixed with a random salt made
// when the file is created, so no two tills share a key. Each row's key is
// its additional data, so a blob moved to another row fails to open. This
// stops anyone from opening the photos with a file manager or an SQLite
// browser; it can't stop someone who takes kashi-pos itself apart, since
// the app has to hold the key.
//
// It's a cache: anything wrong with a row (it won't decrypt, the file is
// gone) is a miss, and the photo is fetched from kashi again.
type photoCache struct {
	db       *sql.DB
	aead     cipher.AEAD
	maxBytes int64
	now      func() time.Time
	puts     int
}

// photoCacheSecret is half of the cache key; the per-file salt is the other.
var photoCacheSecret = []byte("kp-rc/v1:6f1d0a3e9b2c4f7a8e5d1b0c3a9f2e7d")

// photoCacheMaxBytes caps the file. Thumbnails are a few KB and full photos
// a few hundred, so this holds thousands; the least recently shown go first.
const photoCacheMaxBytes = 512 << 20

// photoCacheTouchEvery limits how often a read records its use, so showing
// a page isn't a write per photo.
const photoCacheTouchEvery = time.Hour

type cachedPhoto struct {
	Body        []byte
	ContentType string
	ETag        string
	// Missing records that kashi said there's no photo, so a row without
	// one shows its icon at once instead of asking kashi every time.
	Missing   bool
	CheckedAt time.Time
}

// photoCachePath is where the cache lives: the OS cache directory (local,
// never a roaming profile), in a dot folder hidden on Windows too.
func photoCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kashi-pos", ".rc", "store"), nil
}

func openPhotoCache(path string) (*photoCache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	hidePath(filepath.Dir(path))
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	// One connection: writes are small and rare, and it rules out
	// SQLITE_BUSY between this cache's own goroutines.
	conn.SetMaxOpenConns(1)
	c := &photoCache{db: conn, maxBytes: photoCacheMaxBytes, now: time.Now}
	if err := c.init(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *photoCache) init() error {
	if _, err := c.db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v BLOB NOT NULL);
		CREATE TABLE IF NOT EXISTS entries (
			key          TEXT PRIMARY KEY,
			ref          TEXT NOT NULL,
			etag         TEXT NOT NULL,
			content_type TEXT NOT NULL,
			missing      INTEGER NOT NULL,
			data         BLOB NOT NULL,
			size         INTEGER NOT NULL,
			checked_at   INTEGER NOT NULL,
			used_at      INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS entries_ref ON entries (ref);
		CREATE INDEX IF NOT EXISTS entries_used ON entries (used_at);
	`); err != nil {
		return err
	}
	var salt []byte
	err := c.db.QueryRow(`SELECT v FROM meta WHERE k = 'salt'`).Scan(&salt)
	if errors.Is(err, sql.ErrNoRows) {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		if _, err := c.db.Exec(`INSERT INTO meta (k, v) VALUES ('salt', ?)`, salt); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	key := sha256.Sum256(append(append([]byte{}, photoCacheSecret...), salt...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	c.aead, err = cipher.NewGCM(block)
	return err
}

func (c *photoCache) close() error {
	return c.db.Close()
}

// get returns the copy stored under key. A row that won't decrypt (damaged,
// or from another key) is dropped and reported as a miss.
func (c *photoCache) get(key string) (cachedPhoto, bool) {
	var (
		p               cachedPhoto
		missing         int
		data            []byte
		checked, usedAt int64
	)
	err := c.db.QueryRow(`SELECT etag, content_type, missing, data, checked_at, used_at FROM entries WHERE key = ?`, key).
		Scan(&p.ETag, &p.ContentType, &missing, &data, &checked, &usedAt)
	if err != nil {
		return cachedPhoto{}, false
	}
	p.Missing = missing != 0
	p.CheckedAt = time.UnixMilli(checked)
	if !p.Missing {
		body, err := c.open(key, data)
		if err != nil {
			_, _ = c.db.Exec(`DELETE FROM entries WHERE key = ?`, key)
			return cachedPhoto{}, false
		}
		p.Body = body
	}
	now := c.now()
	if now.Sub(time.UnixMilli(usedAt)) > photoCacheTouchEvery {
		_, _ = c.db.Exec(`UPDATE entries SET used_at = ? WHERE key = ?`, now.UnixMilli(), key)
	}
	return p, true
}

// put stores a copy, checked now. ref names the record it belongs to, so
// forgetRef can drop all of a record's copies at once.
func (c *photoCache) put(key, ref string, p cachedPhoto) error {
	data := []byte{}
	if !p.Missing {
		var err error
		if data, err = c.seal(key, p.Body); err != nil {
			return err
		}
	}
	now := c.now().UnixMilli()
	_, err := c.db.Exec(`
		INSERT INTO entries (key, ref, etag, content_type, missing, data, size, checked_at, used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET ref = excluded.ref, etag = excluded.etag,
			content_type = excluded.content_type, missing = excluded.missing, data = excluded.data,
			size = excluded.size, checked_at = excluded.checked_at, used_at = excluded.used_at`,
		key, ref, p.ETag, p.ContentType, boolInt(p.Missing), data, len(data), now, now)
	if err != nil {
		return err
	}
	c.puts++
	if c.puts%32 == 1 {
		c.evict()
	}
	return nil
}

// markChecked records that kashi confirmed the copy is still current.
func (c *photoCache) markChecked(key string) {
	_, _ = c.db.Exec(`UPDATE entries SET checked_at = ? WHERE key = ?`, c.now().UnixMilli(), key)
}

// forgetRef drops every copy of one record's photos, for when the till
// knows they changed (new photos uploaded).
func (c *photoCache) forgetRef(ref string) {
	if ref == "" {
		return
	}
	_, _ = c.db.Exec(`DELETE FROM entries WHERE ref = ?`, ref)
}

// evict drops the least recently used copies once the file is over its cap,
// down to 80% of it so it isn't trimming on every put.
func (c *photoCache) evict() {
	var total int64
	if err := c.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM entries`).Scan(&total); err != nil || total <= c.maxBytes {
		return
	}
	target := c.maxBytes * 8 / 10
	rows, err := c.db.Query(`SELECT key, size FROM entries ORDER BY used_at, checked_at`)
	if err != nil {
		return
	}
	var drop []string
	for rows.Next() && total > target {
		var key string
		var size int64
		if rows.Scan(&key, &size) == nil {
			drop = append(drop, key)
			total -= size
		}
	}
	rows.Close()
	for _, key := range drop {
		_, _ = c.db.Exec(`DELETE FROM entries WHERE key = ?`, key)
	}
}

func (c *photoCache) seal(key string, body []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, body, []byte(key)), nil
}

func (c *photoCache) open(key string, data []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(data) < n {
		return nil, fmt.Errorf("photo cache: short row")
	}
	return c.aead.Open(nil, data[:n], data[n:], []byte(key))
}
