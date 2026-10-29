package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// fakeKashiCatalog answers the completeness-check routes from an in-memory
// set of kashi clients/products, keyed by uid.
type fakeKashiCatalog struct {
	mu          sync.Mutex
	clients     map[string]map[string]any
	products    map[string]map[string]any
	clientLinks []map[string]any
	fetches     int
}

func newFakeKashiCatalog() *fakeKashiCatalog {
	return &fakeKashiCatalog{clients: map[string]map[string]any{}, products: map[string]map[string]any{}}
}

func (f *fakeKashiCatalog) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Entity string   `json:"entity"`
			UIDs   []string `json:"uids"`
		}
		json.Unmarshal(body, &req)
		set := f.clients
		if req.Entity == "products" {
			set = f.products
		}
		switch r.URL.Path {
		case "/branch/sync/ids":
			ids := []map[string]any{}
			for uid, row := range set {
				ids = append(ids, map[string]any{"uid": uid, "updated_at": row["updated_at"]})
			}
			json.NewEncoder(w).Encode(map[string]any{"ids": ids, "next": ""})
		case "/branch/sync/fetch":
			f.fetches++
			items := []any{}
			for _, uid := range req.UIDs {
				if row, ok := set[uid]; ok {
					items = append(items, row)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"items": items})
		case "/branch/sync/client_uids":
			json.NewEncoder(w).Encode(map[string]any{"pairs": f.clientLinks})
		default:
			w.Write([]byte(`{}`))
		}
	})
}

const (
	fcClientUID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	fcProductUID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

// A client kashi has and the till doesn't is fetched; a second check with
// nothing changed fetches nothing.
func TestFullCheckFetchesMissingAndSkipsCurrent(t *testing.T) {
	fk := newFakeKashiCatalog()
	stamp := time.Date(2026, 9, 1, 10, 0, 0, 123456000, time.UTC).Format(time.RFC3339Nano)
	fk.clients[fcClientUID] = map[string]any{"uid": fcClientUID, "name": "Rana", "phone": "70555111", "is_active": true, "updated_at": stamp}
	server := httptest.NewServer(fk.handler())
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	cc := newCentralClient(server.URL, "TEST01", "test-key")

	if _, err := app.runFullCheckLocked(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetClientByUID(app.ctx, sql.NullString{String: fcClientUID, Valid: true})
	if err != nil {
		t.Fatalf("missing client was not fetched: %v", err)
	}
	if got.Phone != "70555111" || !got.RemoteUpdatedAt.Valid {
		t.Fatalf("unexpected fetched client %+v", got)
	}

	fk.fetches = 0
	if _, err := app.runFullCheckLocked(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if fk.fetches != 0 {
		t.Fatalf("expected no fetch when the till is current, got %d", fk.fetches)
	}
}

// A client this branch had linked before uids existed adopts kashi's uid,
// so kashi's (different) phone updates that same row instead of creating a
// second client.
func TestFullCheckAdoptsLinkedClientUID(t *testing.T) {
	fk := newFakeKashiCatalog()
	server := httptest.NewServer(fk.handler())
	defer server.Close()

	app, store, _, client := setUpTestInvoiceApp(t, server.URL)
	fk.clientLinks = []map[string]any{{"branch_client_id": client.ID, "uid": fcClientUID}}
	fk.clients[fcClientUID] = map[string]any{"uid": fcClientUID, "name": client.Name, "phone": "71999888", "is_active": true,
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano)}

	rep, err := app.runFullCheckLocked(app.ctx, newCentralClient(server.URL, "TEST01", "test-key"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.AdoptedClientUIDs != 1 {
		t.Fatalf("expected 1 adopted uid, got %d", rep.AdoptedClientUIDs)
	}
	got, err := store.GetClientByID(app.ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Uid.String != fcClientUID || got.Phone != "71999888" {
		t.Fatalf("expected linked row to adopt uid and kashi's phone, got %+v", got)
	}
	if _, err := store.GetClientByPhone(app.ctx, client.Phone); err != sql.ErrNoRows {
		t.Fatalf("old phone should no longer exist as a separate client, err=%v", err)
	}
}

// Local products kashi doesn't have are deactivated only once full-mirror
// mode is confirmed; kashi's products are matched by barcode and kept.
func TestFullCheckDeactivatesOrphanProductsOnlyWhenConfirmed(t *testing.T) {
	fk := newFakeKashiCatalog()
	fk.products[fcProductUID] = map[string]any{"uid": fcProductUID, "barcode": "KASHI-1", "code": "K1", "name": "Kashi Shirt",
		"price": map[string]any{"Int64": 1000, "Valid": true}, "is_active": true, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	server := httptest.NewServer(fk.handler())
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	cc := newCentralClient(server.URL, "TEST01", "test-key")
	local, err := store.GetProductByBarcode(app.ctx, "1") // the fixture's till-only product
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.runFullCheckLocked(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if p, _ := store.GetProductByID(app.ctx, local.ID); p.IsActive != 1 {
		t.Fatal("must not deactivate before full-mirror mode is confirmed")
	}

	if err := store.SetFullMirrorConfirmedAt(app.ctx, sql.NullString{String: "2026-01-01T00:00:00Z", Valid: true}); err != nil {
		t.Fatal(err)
	}
	rep, err := app.runFullCheckLocked(app.ctx, cc)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := store.GetProductByID(app.ctx, local.ID); p.IsActive != 0 {
		t.Fatal("expected till-only product deactivated once confirmed")
	}
	kashiProduct, err := store.GetProductByUID(app.ctx, sql.NullString{String: fcProductUID, Valid: true})
	if err != nil || kashiProduct.IsActive != 1 {
		t.Fatalf("kashi's product must exist and stay active: %+v err=%v", kashiProduct, err)
	}
	for _, e := range rep.Entities {
		if e.Entity == "products" && e.Deactivated != 1 {
			t.Fatalf("expected 1 deactivated product, got %+v", e)
		}
	}
}

// The built-in "Unknown item" placeholder is the till's own, so kashi never
// has its uid; the full check must leave it on. One an earlier version
// switched off is turned back on by the Unknown item button.
func TestFullCheckKeepsTheUnknownItemPlaceholder(t *testing.T) {
	fk := newFakeKashiCatalog()
	fk.products[fcProductUID] = map[string]any{"uid": fcProductUID, "barcode": "KASHI-1", "code": "K1", "name": "Kashi Shirt",
		"price": map[string]any{"Int64": 1000, "Valid": true}, "is_active": true, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	server := httptest.NewServer(fk.handler())
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	cc := newCentralClient(server.URL, "TEST01", "test-key")
	placeholder, err := ensureUnknownProduct(app.ctx, store.Queries, defaultUnknownBarcode)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFullMirrorConfirmedAt(app.ctx, sql.NullString{String: "2026-01-01T00:00:00Z", Valid: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.runFullCheckLocked(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if p, _ := store.GetProductByID(app.ctx, placeholder.ID); p.IsActive != 1 {
		t.Fatal("the full check switched the Unknown item placeholder off")
	}

	if err := store.SetProductActive(app.ctx, db.SetProductActiveParams{IsActive: 0, ID: placeholder.ID}); err != nil {
		t.Fatal(err)
	}
	if p, err := ensureUnknownProduct(app.ctx, store.Queries, defaultUnknownBarcode); err != nil || p.IsActive != 1 {
		t.Fatalf("placeholder not turned back on: %+v err=%v", p, err)
	}
	if p, _ := store.GetProductByID(app.ctx, placeholder.ID); p.IsActive != 1 {
		t.Fatal("placeholder still off in the database")
	}
}

// A till that already holds kashi's client (matching stamp) and has no
// cursor pulls nothing in full: the missing client comes through the uid
// check, the held one is skipped, and the cursor is seeded from kashi's clock.
func TestPullSeedsCursorFromLocalCatalog(t *testing.T) {
	fk := newFakeKashiCatalog()
	stamp := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	fk.clients[fcClientUID] = map[string]any{"uid": fcClientUID, "name": "Rana", "phone": "70555111", "is_active": true, "updated_at": stamp}
	serverTime := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	var fullPulls int
	inner := fk.handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/branch/sync/changes" {
			inner.ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("since") == "" {
			fullPulls++
		}
		json.NewEncoder(w).Encode(map[string]any{"entity": "clients", "items": []any{}, "server_time": serverTime})
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	cc := newCentralClient(server.URL, "TEST01", "test-key")
	if _, err := app.runFullCheckLocked(app.ctx, cc); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSyncCursor(app.ctx, db.SetSyncCursorParams{Entity: "clients", Cursor: ""}); err != nil {
		t.Fatal(err)
	}
	// kashi gains a second client the till has never seen.
	other := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	fk.clients[other] = map[string]any{"uid": other, "name": "Omar", "phone": "70555222", "is_active": true, "updated_at": stamp}
	fk.fetches = 0

	if _, err := app.pullEntityLocked(app.ctx, cc, "clients"); err != nil {
		t.Fatal(err)
	}
	if fullPulls != 0 {
		t.Fatalf("expected no from-the-beginning pull, got %d", fullPulls)
	}
	if _, err := store.GetClientByUID(app.ctx, sql.NullString{String: other, Valid: true}); err != nil {
		t.Fatalf("missing client was not fetched: %v", err)
	}
	cursor, err := store.GetSyncCursor(app.ctx, "clients")
	if err != nil || cursor != serverTime.Format(time.RFC3339) {
		t.Fatalf("cursor = %q, %v", cursor, err)
	}
}
