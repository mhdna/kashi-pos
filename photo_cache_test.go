package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// ---- the store ----------------------------------------------------------

type photoClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *photoClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *photoClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func openTestPhotoCache(t *testing.T, path string) (*photoCache, *photoClock) {
	t.Helper()
	cache, err := openPhotoCache(path)
	if err != nil {
		t.Fatal(err)
	}
	clock := &photoClock{t: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)}
	cache.now = clock.now
	t.Cleanup(func() { cache.close() })
	return cache, clock
}

// rawCacheBytes is everything SQLite wrote for the cache, WAL included.
func rawCacheBytes(t *testing.T, path string) []byte {
	t.Helper()
	var all []byte
	for _, p := range []string{path, path + "-wal"} {
		if data, err := os.ReadFile(p); err == nil {
			all = append(all, data...)
		}
	}
	return all
}

// A photo comes back exactly as saved, after a restart too, while nothing
// readable is on disk: no image bytes, no image file to open.
func TestPhotoCacheStoresEncrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rc", "store")
	cache, _ := openTestPhotoCache(t, path)
	photo := append([]byte("\x89PNG\r\n\x1a\n"), []byte("RECEIPT-PIXELS-CENTRO-EXPENSE-42")...)
	if err := cache.put("k1", "exp-1", cachedPhoto{Body: photo, ContentType: "image/png", ETag: `"a-w96-r0"`}); err != nil {
		t.Fatal(err)
	}
	got, ok := cache.get("k1")
	if !ok || !bytes.Equal(got.Body, photo) || got.ContentType != "image/png" || got.ETag != `"a-w96-r0"` {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	raw := rawCacheBytes(t, path)
	if bytes.Contains(raw, []byte("RECEIPT-PIXELS")) || bytes.Contains(raw, []byte("\x89PNG")) {
		t.Fatal("the photo is readable in the cache file")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if filepath.Ext(e.Name()) != "" && e.Name() != "store-wal" && e.Name() != "store-shm" {
			t.Fatalf("unexpected file %s next to the cache", e.Name())
		}
	}

	cache.close()
	reopened, _ := openTestPhotoCache(t, path)
	got, ok = reopened.get("k1")
	if !ok || !bytes.Equal(got.Body, photo) {
		t.Fatal("a reopened cache must read its own photos")
	}
}

// The app's secret is half the key: the same file opened by a build with
// another secret reads nothing, and drops what it can't open.
func TestPhotoCacheNeedsTheAppSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store")
	cache, _ := openTestPhotoCache(t, path)
	if err := cache.put("k1", "exp-1", cachedPhoto{Body: []byte("photo"), ContentType: "image/webp"}); err != nil {
		t.Fatal(err)
	}
	cache.close()

	old := photoCacheSecret
	photoCacheSecret = []byte("someone else's build")
	defer func() { photoCacheSecret = old }()
	other, _ := openTestPhotoCache(t, path)
	if _, ok := other.get("k1"); ok {
		t.Fatal("opened with the wrong key")
	}
	var n int
	other.db.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&n)
	if n != 0 {
		t.Fatal("a row that won't open should be dropped")
	}
}

// A row's blob moved onto another row, or damaged, is a miss, never another
// record's photo.
func TestPhotoCacheRowsCantBeSwappedOrDamaged(t *testing.T) {
	cache, _ := openTestPhotoCache(t, filepath.Join(t.TempDir(), "store"))
	cache.put("a", "exp-a", cachedPhoto{Body: []byte("photo A"), ContentType: "image/webp"})
	cache.put("b", "exp-b", cachedPhoto{Body: []byte("photo B"), ContentType: "image/webp"})
	if _, err := cache.db.Exec(`UPDATE entries SET data = (SELECT data FROM entries WHERE key = 'a') WHERE key = 'b'`); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.get("b"); ok {
		t.Fatal("b opened with a's blob")
	}
	if got, ok := cache.get("a"); !ok || string(got.Body) != "photo A" {
		t.Fatal("a must be untouched")
	}
	cache.db.Exec(`UPDATE entries SET data = substr(data, 1, length(data) - 1) || x'00' WHERE key = 'a'`)
	if _, ok := cache.get("a"); ok {
		t.Fatal("a damaged row opened")
	}
}

// "No photo" is remembered too, and forgetRef drops only that record.
func TestPhotoCacheMissingAndForget(t *testing.T) {
	cache, _ := openTestPhotoCache(t, filepath.Join(t.TempDir(), "store"))
	cache.put("thumb", "exp-1", cachedPhoto{Missing: true})
	cache.put("full", "exp-1", cachedPhoto{Body: []byte("x"), ContentType: "image/webp"})
	cache.put("other", "exp-2", cachedPhoto{Body: []byte("y"), ContentType: "image/webp"})
	if got, ok := cache.get("thumb"); !ok || !got.Missing || got.Body != nil {
		t.Fatalf("missing: %+v %v", got, ok)
	}
	cache.forgetRef("exp-1")
	if _, ok := cache.get("thumb"); ok {
		t.Fatal("thumb kept")
	}
	if _, ok := cache.get("full"); ok {
		t.Fatal("full kept")
	}
	if _, ok := cache.get("other"); !ok {
		t.Fatal("another record's photo was dropped")
	}
	cache.forgetRef("") // a record without a ref drops nothing
	if _, ok := cache.get("other"); !ok {
		t.Fatal("forgetRef(\"\") dropped photos")
	}
}

func cacheSize(t *testing.T, cache *photoCache) int64 {
	var total int64
	if err := cache.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM entries`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	return total
}

// Over the cap, the photos shown longest ago go first; the ones in use stay.
func TestPhotoCacheEvictsLeastRecentlyShown(t *testing.T) {
	cache, clock := openTestPhotoCache(t, filepath.Join(t.TempDir(), "store"))
	cache.maxBytes = 10_000
	body := bytes.Repeat([]byte{7}, 1000)
	for i := 0; i < 8; i++ {
		cache.put(fmt.Sprint("old-", i), "r", cachedPhoto{Body: body, ContentType: "image/webp"})
		clock.advance(2 * time.Hour)
	}
	// old-0 is shown again, so it's recent now.
	if _, ok := cache.get("old-0"); !ok {
		t.Fatal("old-0 missing")
	}
	clock.advance(2 * time.Hour)
	for i := 0; i < 40; i++ {
		cache.put(fmt.Sprint("new-", i), "r", cachedPhoto{Body: body, ContentType: "image/webp"})
		clock.advance(time.Minute)
	}
	cache.evict()
	if got := cacheSize(t, cache); got > cache.maxBytes {
		t.Fatalf("cache is %d bytes, over its %d cap", got, cache.maxBytes)
	}
	if _, ok := cache.get("new-39"); !ok {
		t.Fatal("the newest photo was evicted")
	}
	if _, ok := cache.get("old-1"); ok {
		t.Fatal("the least recently shown photo is still there")
	}
}

// Hundreds of random saves, reads, forgets and evictions: every read is
// exactly what was last saved under that key, or a miss -- never another
// photo or a damaged one -- and the file stays under its cap.
func TestPhotoCacheRandomized(t *testing.T) {
	cache, clock := openTestPhotoCache(t, filepath.Join(t.TempDir(), "store"))
	cache.maxBytes = 50_000
	rng := rand.New(rand.NewSource(20261110))
	want := map[string][]byte{}
	refs := map[string]string{}
	for i := 0; i < 600; i++ {
		key := fmt.Sprint("k", rng.Intn(60))
		switch op := rng.Intn(10); {
		case op < 5:
			body := make([]byte, 1+rng.Intn(4000))
			rng.Read(body)
			ref := fmt.Sprint("ref-", rng.Intn(10))
			if err := cache.put(key, ref, cachedPhoto{Body: body, ContentType: "image/webp"}); err != nil {
				t.Fatal(err)
			}
			want[key], refs[key] = body, ref
		case op < 9:
			got, ok := cache.get(key)
			if ok && !bytes.Equal(got.Body, want[key]) {
				t.Fatalf("step %d: %s came back different from what was saved", i, key)
			}
		default:
			ref := fmt.Sprint("ref-", rng.Intn(10))
			cache.forgetRef(ref)
			for k, r := range refs {
				if r == ref {
					delete(want, k)
					delete(refs, k)
				}
			}
		}
		clock.advance(time.Duration(rng.Intn(120)) * time.Minute)
	}
	cache.evict()
	if got := cacheSize(t, cache); got > cache.maxBytes {
		t.Fatalf("cache is %d bytes, over its %d cap", got, cache.maxBytes)
	}
	for key := range refs {
		if got, ok := cache.get(key); ok && !bytes.Equal(got.Body, want[key]) {
			t.Fatalf("%s is wrong after the run", key)
		}
	}
}

// ---- fetching through the cache -----------------------------------------

// fakePhotoKashi serves photos like kashi's serveStoredImage: an ETag per
// file, 304 for a matching If-None-Match, 404 for no photo.
type fakePhotoKashi struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	photos map[string]fakePhoto // by path + query
	hits   map[string]int
	gate   chan struct{} // when set, requests wait on it
}

type fakePhoto struct {
	body []byte
	etag string
}

func newFakePhotoKashi(t *testing.T) *fakePhotoKashi {
	k := &fakePhotoKashi{t: t, photos: map[string]fakePhoto{}, hits: map[string]int{}}
	k.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k.mu.Lock()
		gate := k.gate
		k.mu.Unlock()
		if gate != nil {
			<-gate
		}
		k.mu.Lock()
		defer k.mu.Unlock()
		key := r.URL.RequestURI()
		k.hits[key]++
		photo, ok := k.photos[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if photo.etag != "" {
			w.Header().Set("ETag", photo.etag)
			if r.Header.Get("If-None-Match") == photo.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Write(photo.body)
	}))
	t.Cleanup(k.server.Close)
	return k
}

func (k *fakePhotoKashi) set(path string, body string, etag string) {
	k.mu.Lock()
	k.photos[path] = fakePhoto{body: []byte(body), etag: etag}
	k.mu.Unlock()
}

func (k *fakePhotoKashi) remove(path string) {
	k.mu.Lock()
	delete(k.photos, path)
	k.mu.Unlock()
}

func (k *fakePhotoKashi) hitCount(path string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.hits[path]
}

func (k *fakePhotoKashi) hold() {
	k.mu.Lock()
	k.gate = make(chan struct{})
	k.mu.Unlock()
}

func (k *fakePhotoKashi) release() {
	k.mu.Lock()
	close(k.gate)
	k.gate = nil
	k.mu.Unlock()
}

type photoTestApp struct {
	*App
	clock   *photoClock
	mu      sync.Mutex
	changed []string
}

func (p *photoTestApp) changes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.changed...)
}

// newPhotoTestApp is a till pointed at kashiURL as branch code, with its
// photo cache in a temp folder.
func newPhotoTestApp(t *testing.T, kashiURL, code string) *photoTestApp {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(dir, "till.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, receipt_enabled, sales_inventory_id, central_api_url, branch_code
	) VALUES ('Test', 'key', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, ?, ?)`, kashiURL, code); err != nil {
		t.Fatal(err)
	}
	cache, clock := openTestPhotoCache(t, filepath.Join(dir, ".rc", "store"))
	p := &photoTestApp{App: &App{ctx: ctx, store: db.NewStore(database), conn: database, ready: make(chan struct{})}, clock: clock}
	p.photoCache = cache
	p.onPhotosChanged = func(ref string) {
		p.mu.Lock()
		p.changed = append(p.changed, ref)
		p.mu.Unlock()
	}
	t.Cleanup(p.photoWG.Wait)
	return p
}

func dataURL(body string) string {
	return "data:image/webp;base64," + base64.StdEncoding.EncodeToString([]byte(body))
}

const firstThumb = "/branch/expense_photos/first?client_ref=exp-1&w=96"

// The case asked for: a photo seen once comes from disk after that, with no
// wait on kashi, even while kashi is slow to answer.
func TestExpensePhotoSecondShowingIsFromDisk(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb v1", `"f1-w96-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")

	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb v1") {
		t.Fatalf("first showing = %q", got)
	}
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb v1") {
		t.Fatalf("second showing = %q", got)
	}
	if n := kashi.hitCount(firstThumb); n != 1 {
		t.Fatalf("kashi asked %d times, want 1 (the second showing is from disk)", n)
	}

	// Later, with kashi hanging: the copy still shows at once.
	app.clock.advance(photoRecheckAfter)
	kashi.hold()
	start := time.Now()
	got, _ := app.GetExpenseFirstPhoto("exp-1", 96)
	if got != dataURL("thumb v1") || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("a due copy must still show at once, got %q after %s", got, time.Since(start))
	}
	kashi.release()
	app.photoWG.Wait()
	if n := kashi.hitCount(firstThumb); n != 2 {
		t.Fatalf("kashi asked %d times, want 2 (one background check)", n)
	}
	if c := app.changes(); len(c) != 0 {
		t.Fatalf("an unchanged photo announced a change: %v", c)
	}
	// The check counted: the next showing doesn't ask again.
	app.GetExpenseFirstPhoto("exp-1", 96)
	app.photoWG.Wait()
	if n := kashi.hitCount(firstThumb); n != 2 {
		t.Fatalf("asked again right after a check (%d)", n)
	}
}

// A photo turned or replaced in kashi (new ETag) replaces the copy and tells
// the page which record to redraw.
func TestExpensePhotoChangedInKashiIsPickedUp(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb v1", `"f1-w96-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.GetExpenseFirstPhoto("exp-1", 96)

	kashi.set(firstThumb, "thumb turned", `"f1-w96-r90"`)
	app.clock.advance(photoRecheckAfter)
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb v1") {
		t.Fatalf("shows %q before the check", got)
	}
	app.photoWG.Wait()
	if c := app.changes(); len(c) != 1 || c[0] != "exp-1" {
		t.Fatalf("changes = %v, want [exp-1]", c)
	}
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb turned") {
		t.Fatalf("after the change = %q", got)
	}
}

// A photo deleted in kashi turns into the no-photo icon.
func TestExpensePhotoRemovedInKashi(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb v1", `"f1-w96-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.GetExpenseFirstPhoto("exp-1", 96)

	kashi.remove(firstThumb)
	app.clock.advance(photoRecheckAfter)
	app.GetExpenseFirstPhoto("exp-1", 96)
	app.photoWG.Wait()
	if c := app.changes(); len(c) != 1 || c[0] != "exp-1" {
		t.Fatalf("changes = %v", c)
	}
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != "" {
		t.Fatalf("a removed photo still shows: %q", got)
	}
}

// An expense without a photo shows its icon at once the next time, and a
// later recheck that still finds none announces nothing.
func TestExpenseWithoutPhotoIsRemembered(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	for i := 0; i < 3; i++ {
		if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != "" {
			t.Fatalf("got %q for no photo", got)
		}
	}
	if n := kashi.hitCount(firstThumb); n != 1 {
		t.Fatalf("asked %d times, want 1", n)
	}
	app.clock.advance(photoRecheckAfter)
	app.GetExpenseFirstPhoto("exp-1", 96)
	app.photoWG.Wait()
	if c := app.changes(); len(c) != 0 {
		t.Fatalf("still no photo, but announced %v", c)
	}
}

// New photos arriving (the phone's upload push) drop the remembered "no
// photo", so the row shows the new one straight away.
func TestUploadedPhotosReplaceTheRememberedNone(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.GetExpenseFirstPhoto("exp-1", 96)

	kashi.set(firstThumb, "new photo", `"f2-w96-r0"`)
	app.handleWSMessage(nil, []byte(`{"type":"expense_images_uploaded","client_ref":"exp-1"}`))
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("new photo") {
		t.Fatalf("after the upload push = %q", got)
	}

	// Same for a loan.
	loanThumb := "/branch/loan_photos/first?client_ref=loan-1&w=96"
	app.GetLoanFirstPhoto("loan-1", 96)
	kashi.set(loanThumb, "loan photo", `"l1-w96-r0"`)
	app.handleWSMessage(nil, []byte(`{"type":"loan_images_uploaded","client_ref":"loan-1"}`))
	if got, _ := app.GetLoanFirstPhoto("loan-1", 96); got != dataURL("loan photo") {
		t.Fatalf("loan after the upload push = %q", got)
	}
}

// Offline, saved photos still show; unsaved ones are the icon, and nothing
// is remembered from a failed fetch.
func TestPhotosOffline(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb v1", `"f1-w96-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.GetExpenseFirstPhoto("exp-1", 96)
	kashi.server.Close()

	app.clock.advance(photoRecheckAfter)
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb v1") {
		t.Fatalf("offline = %q", got)
	}
	if got, _ := app.GetExpenseFirstPhoto("exp-2", 96); got != "" {
		t.Fatalf("never seen, offline = %q", got)
	}
	app.photoWG.Wait()
	if _, ok := app.photoCache.get(photoCacheKey(newCentralClient(kashi.server.URL, "CN", "key"), "/branch/expense_photos/first?client_ref=exp-2&w=96")); ok {
		t.Fatal("a failed fetch was saved as no photo")
	}
	if c := app.changes(); len(c) != 0 {
		t.Fatalf("a failed check announced %v", c)
	}
}

// Copies are per kashi and branch: a till moved to another branch never
// shows the old branch's photo for the same request.
func TestPhotoCopiesArePerBranch(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb", `"f1-w96-r0"`)
	client := newCentralClient(kashi.server.URL, "CN", "key")
	if photoCacheKey(client, firstThumb) == photoCacheKey(newCentralClient(kashi.server.URL, "HR", "key"), firstThumb) {
		t.Fatal("two branches share a copy")
	}
	if photoCacheKey(client, firstThumb) == photoCacheKey(newCentralClient("https://other.example", "CN", "key"), firstThumb) {
		t.Fatal("two servers share a copy")
	}
	if photoCacheKey(client, firstThumb) == photoCacheKey(client, "/branch/expense_photos/first?client_ref=exp-1&w=1280") {
		t.Fatal("a thumbnail and the full photo share a copy")
	}
}

// Many rows asking at once while a check is out make one request, not one
// each.
func TestPhotoRecheckRunsOncePerCopy(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb", `"f1-w96-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.GetExpenseFirstPhoto("exp-1", 96)
	app.clock.advance(photoRecheckAfter)
	kashi.hold()
	for i := 0; i < 20; i++ {
		app.GetExpenseFirstPhoto("exp-1", 96)
	}
	kashi.release()
	app.photoWG.Wait()
	if n := kashi.hitCount(firstThumb); n != 2 {
		t.Fatalf("kashi asked %d times, want 2", n)
	}
}

// The full photos in a record's detail come by id, named after the record so
// a change redraws that record.
func TestExpenseAndLoanPhotoByID(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set("/branch/expense_photos/7/file?w=1280", "expense full", `"e7-w1280-r0"`)
	kashi.set("/branch/loan_photos/8/file?w=1280", "loan full", `"l8-w1280-r0"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	if got, _ := app.GetExpensePhoto("exp-1", 7, 1280); got != dataURL("expense full") {
		t.Fatalf("expense = %q", got)
	}
	if got, _ := app.GetLoanPhoto("loan-1", 8, 1280); got != dataURL("loan full") {
		t.Fatalf("loan = %q", got)
	}
	kashi.set("/branch/loan_photos/8/file?w=1280", "loan replaced", `"l8b-w1280-r0"`)
	app.clock.advance(photoRecheckAfter)
	app.GetLoanPhoto("loan-1", 8, 1280)
	app.photoWG.Wait()
	if c := app.changes(); len(c) != 1 || c[0] != "loan-1" {
		t.Fatalf("changes = %v", c)
	}
	if got, _ := app.GetExpensePhoto("exp-1", 0, 1280); got != "" {
		t.Fatal("id 0 must be no photo")
	}
}

// Transfers are named by the till's clientRef when it sent them, by
// "transfer-<id>" when it receives them.
func TestTransferPhotoCachedWithItsRef(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	in := "/branch/transfer_photo?transfer_id=5&w=96"
	out := "/branch/transfer_photo?client_ref=tr-out&w=96"
	kashi.set(in, "received", `"t5"`)
	kashi.set(out, "sent", `"t6"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	if got, _ := app.GetTransferPhoto("", 5, 96); got != dataURL("received") {
		t.Fatalf("in = %q", got)
	}
	if got, _ := app.GetTransferPhoto("tr-out", 0, 96); got != dataURL("sent") {
		t.Fatalf("out = %q", got)
	}
	app.GetTransferPhoto("", 5, 96)
	if kashi.hitCount(in) != 1 {
		t.Fatal("the received transfer's photo wasn't served from disk")
	}
	kashi.set(in, "received, turned", `"t5-r90"`)
	kashi.set(out, "sent, turned", `"t6-r90"`)
	app.clock.advance(photoRecheckAfter)
	app.GetTransferPhoto("", 5, 96)
	app.GetTransferPhoto("tr-out", 0, 96)
	app.photoWG.Wait()
	got := map[string]bool{}
	for _, ref := range app.changes() {
		got[ref] = true
	}
	if !got["transfer-5"] || !got["tr-out"] || len(got) != 2 {
		t.Fatalf("changes = %v", app.changes())
	}
	if got, _ := app.GetTransferPhoto("", 0, 96); got != "" {
		t.Fatal("no ref, no id must be no photo")
	}
}

// The photo id list always asks kashi while it's there -- the upload dialog
// polls it to see a new photo land -- and falls back to the saved list only
// offline.
func TestPhotoIDListIsLiveWithOfflineFallback(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	list := "/branch/expense_photos?client_ref=exp-1"
	body, _ := json.Marshal(map[string]any{"images": []map[string]int64{{"id": 7}}})
	kashi.set(list, string(body), "")
	app := newPhotoTestApp(t, kashi.server.URL, "CN")

	ids, _ := app.ListExpensePhotoIDs("exp-1")
	if fmt.Sprint(ids) != "[7]" {
		t.Fatalf("ids = %v", ids)
	}
	body, _ = json.Marshal(map[string]any{"images": []map[string]int64{{"id": 7}, {"id": 9}}})
	kashi.set(list, string(body), "")
	ids, _ = app.ListExpensePhotoIDs("exp-1")
	if fmt.Sprint(ids) != "[7 9]" || kashi.hitCount(list) != 2 {
		t.Fatalf("a new photo must show on the next poll: ids %v after %d asks", ids, kashi.hitCount(list))
	}

	kashi.server.Close()
	ids, _ = app.ListExpensePhotoIDs("exp-1")
	if fmt.Sprint(ids) != "[7 9]" {
		t.Fatalf("offline ids = %v, want the saved list", ids)
	}
	if ids, _ := app.ListLoanPhotoIDs("never-seen"); len(ids) != 0 {
		t.Fatalf("never seen, offline = %v", ids)
	}
}

// With no cache (it couldn't be opened), photos still come from kashi.
func TestPhotosWithoutACache(t *testing.T) {
	kashi := newFakePhotoKashi(t)
	kashi.set(firstThumb, "thumb", `"f1"`)
	app := newPhotoTestApp(t, kashi.server.URL, "CN")
	app.photoCache.close()
	app.photoCache = nil
	app.photoCacheOnce.Do(func() {}) // as if opening had failed
	if got, _ := app.GetExpenseFirstPhoto("exp-1", 96); got != dataURL("thumb") {
		t.Fatalf("got %q", got)
	}
	app.forgetPhotos("exp-1") // no cache: nothing to drop, no crash
}
