package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

// Central's starting options (kashi migrations 000260 to 000262), under the
// same uids.
var centralSeedSources = []ClientSource{
	{UID: "6f1c2a00-5e1d-4c7a-9b00-000000000001", Name: "Social media"},
	{UID: "6f1c2a00-5e1d-4c7a-9b00-000000000003", Name: "A friend or family"},
	{UID: "6f1c2a00-5e1d-4c7a-9b00-000000000004", Name: "Walking by"},
}

func mustSources(t *testing.T, app *App) []ClientSource {
	t.Helper()
	got, err := app.GetClientSources()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A till asks before its first sync, from central's starting options.
func TestClientSourcesSeededLikeCentral(t *testing.T) {
	app, _, _, _ := setUpTestInvoiceApp(t, "")
	if got := mustSources(t, app); !reflect.DeepEqual(got, centralSeedSources) {
		t.Fatalf("seeded options = %+v, want %+v", got, centralSeedSources)
	}
}

// The reported case: a cashier adds a new customer and picks where they
// heard about us; the answer is kept and goes to central with the client.
func TestCreateClientKeepsWhereTheyHeard(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "https://central.test")
	ctx := context.Background()
	sms := centralSeedSources[2].UID

	created, err := app.CreateClient("Maya", "70456789", sms)
	if err != nil {
		t.Fatal(err)
	}
	id := created["id"].(int64)
	c, err := store.GetClientByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if c.SourceUid != sms {
		t.Fatalf("source_uid = %q, want %q", c.SourceUid, sms)
	}

	// It goes up with the client at their first purchase.
	if err := queueBranchClientSyncForPurchase(ctx, store.Queries, "https://central.test", id); err != nil {
		t.Fatal(err)
	}
	entries, err := store.ListDueSyncOutboxEntries(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	payload := ""
	for _, e := range entries {
		if e.ClientRef == fmt.Sprintf("client-%d", id) {
			payload = e.PayloadJson
		}
	}
	var sent branchClientPayload
	if err := json.Unmarshal([]byte(payload), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.SourceUID != sms {
		t.Fatalf("pushed source_uid = %q, want %q (%s)", sent.SourceUID, sms, payload)
	}

	t.Run("no answer, or an option that's gone, is saved as none", func(t *testing.T) {
		for i, uid := range []string{"", "  ", "0b7e1f7c-3a2d-4f55-9d0c-6a1f2b3c4d5e"} {
			got, err := app.CreateClient("Ali", fmt.Sprintf("7012%04d", 1000+i), uid)
			if err != nil {
				t.Fatalf("%q: %v", uid, err)
			}
			c, _ := store.GetClientByID(ctx, got["id"].(int64))
			if c.SourceUid != "" {
				t.Fatalf("%q saved as %q", uid, c.SourceUid)
			}
		}
	})

	t.Run("a client without an answer pushes none", func(t *testing.T) {
		got, err := app.CreateClient("Omar", "70129999", "")
		if err != nil {
			t.Fatal(err)
		}
		c, _ := store.GetClientByID(ctx, got["id"].(int64))
		body, _ := json.Marshal(branchClientPayload{BranchClientID: c.ID, Name: c.Name, Phone: c.Phone, SourceUID: c.SourceUid})
		if string(body) != fmt.Sprintf(`{"branch_client_id":%d,"name":"Omar","phone":"%s"}`, c.ID, c.Phone) {
			t.Fatalf("payload %s", body)
		}
	})
}

// Central's list replaces the till's: renamed, reordered, hidden and removed
// options all show as central has them, and a removed option leaves the
// customers who gave it alone.
func TestApplySyncedClientSources(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	ctx := context.Background()
	q := store.Queries
	friend := centralSeedSources[2].UID // left out of central's list below
	hiddenUID := "11111111-aaaa-4bbb-8ccc-000000000003"
	c, err := app.CreateClient("Rami", "70111222", friend)
	if err != nil {
		t.Fatal(err)
	}

	err = applySyncedClientSources(ctx, q, []syncedClientSource{
		{UID: centralSeedSources[1].UID, Name: "Text message", SortOrder: 1, IsActive: true},
		{UID: centralSeedSources[0].UID, Name: "Instagram", SortOrder: 2, IsActive: true},
		{UID: hiddenUID, Name: "Other", SortOrder: 3, IsActive: false},
		{UID: "0b7e1f7c-3a2d-4f55-9d0c-6a1f2b3c4d5e", Name: " Radio ", SortOrder: 4, IsActive: true},
		{UID: "", Name: "No uid", IsActive: true},
		{UID: "11111111-2222-4333-8444-555555555555", Name: "  ", IsActive: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []ClientSource{
		{UID: centralSeedSources[1].UID, Name: "Text message"},
		{UID: centralSeedSources[0].UID, Name: "Instagram"},
		{UID: "0b7e1f7c-3a2d-4f55-9d0c-6a1f2b3c4d5e", Name: "Radio"},
	}
	if got := mustSources(t, app); !reflect.DeepEqual(got, want) {
		t.Fatalf("offered = %+v, want %+v", got, want)
	}
	all, _ := q.ListClientSources(ctx)
	if len(all) != 4 {
		t.Fatalf("stored %d options, want 4 (hidden kept)", len(all))
	}
	kept, _ := store.GetClientByID(ctx, c["id"].(int64))
	if kept.SourceUid != friend {
		t.Fatalf("removed option cleared the customer's answer: %q", kept.SourceUid)
	}
	// A hidden option is no answer for a new customer.
	got, _ := app.CreateClient("Lea", "70111333", hiddenUID)
	if hidden, _ := store.GetClientByID(ctx, got["id"].(int64)); hidden.SourceUid != hiddenUID {
		// Hidden still exists locally; the form never offers it, but a
		// form opened before the sync may still send it, and that's kept.
		t.Fatalf("hidden option dropped: %q", hidden.SourceUid)
	}

	if err := applySyncedClientSources(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	if got := mustSources(t, app); len(got) != 0 {
		t.Fatalf("central has none, till offers %+v", got)
	}
}

// Any sequence of lists central might send: after each, the till holds
// exactly central's list and offers exactly its active options in order.
func TestApplySyncedClientSourcesAnyList(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	ctx := context.Background()
	rng := rand.New(rand.NewSource(5))
	pool := make([]string, 12)
	for i := range pool {
		pool[i] = fmt.Sprintf("6f1c2a00-5e1d-4c7a-9b00-%012d", i+1)
	}
	for run := 0; run < 300; run++ {
		var items []syncedClientSource
		for _, i := range rng.Perm(len(pool))[:rng.Intn(len(pool)+1)] {
			items = append(items, syncedClientSource{
				UID: pool[i], Name: fmt.Sprintf("Option %d.%d", i, rng.Intn(3)),
				SortOrder: int64(rng.Intn(5)), IsActive: rng.Intn(4) != 0,
			})
		}
		if err := applySyncedClientSources(ctx, store.Queries, items); err != nil {
			t.Fatal(err)
		}
		all, err := store.ListClientSources(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != len(items) {
			t.Fatalf("run %d: stored %d, central %d", run, len(all), len(items))
		}
		byUID := map[string]syncedClientSource{}
		for _, it := range items {
			byUID[it.UID] = it
		}
		for _, row := range all {
			it, ok := byUID[row.Uid]
			if !ok || it.Name != row.Name || it.SortOrder != row.SortOrder || (row.IsActive == 1) != it.IsActive {
				t.Fatalf("run %d: stored %+v, central %+v", run, row, it)
			}
		}
		offered := mustSources(t, app)
		var want []ClientSource
		for _, row := range all {
			if row.IsActive == 1 {
				want = append(want, ClientSource{UID: row.Uid, Name: row.Name})
			}
		}
		if len(want) == 0 {
			want = []ClientSource{}
		}
		if !reflect.DeepEqual(offered, want) {
			t.Fatalf("run %d: offered %+v, want %+v", run, offered, want)
		}
		for i := 1; i < len(all); i++ {
			if all[i-1].SortOrder > all[i].SortOrder {
				t.Fatalf("run %d: out of order", run)
			}
		}
	}
}

// The whole path: central's feed is pulled and applied; an answer without
// a list keeps the till's options.
func TestPullClientSourcesFromCentral(t *testing.T) {
	var answer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/branch/sync/changes" || r.URL.Query().Get("entity") != "client_sources" {
			w.Write([]byte(`{}`))
			return
		}
		fmt.Fprintf(w, `{"entity":"client_sources",%s"server_time":%q}`, answer, time.Now().UTC().Format(time.RFC3339))
	}))
	defer srv.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, srv.URL)
	cc := newCentralClient(srv.URL, "TEST01", "test-key")
	ctx := context.Background()

	answer = `"items":[{"id":7,"uid":"6f1c2a00-5e1d-4c7a-9b00-000000000003","name":"A friend","sort_order":1,"is_active":true,"created_at":"2026-11-27T10:00:00Z","updated_at":"2026-11-27T10:00:00Z"},{"id":8,"uid":"0b7e1f7c-3a2d-4f55-9d0c-6a1f2b3c4d5e","name":"Radio","sort_order":2,"is_active":true}],`
	if err := app.pullEntityOnce(ctx, cc, "client_sources"); err != nil {
		t.Fatal(err)
	}
	want := []ClientSource{{UID: "6f1c2a00-5e1d-4c7a-9b00-000000000003", Name: "A friend"}, {UID: "0b7e1f7c-3a2d-4f55-9d0c-6a1f2b3c4d5e", Name: "Radio"}}
	if got := mustSources(t, app); !reflect.DeepEqual(got, want) {
		t.Fatalf("after pull = %+v, want %+v", got, want)
	}

	for _, broken := range []string{``, `"items":null,`} {
		answer = broken
		_ = app.pullEntityOnce(ctx, cc, "client_sources") // an error is fine; losing the list isn't
		if got := mustSources(t, app); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q changed the options: %+v", broken, got)
		}
	}
	if got := mustSources(t, app); !reflect.DeepEqual(got, want) {
		t.Fatalf("an answer without a list changed the options: %+v", got)
	}
}

// The new-customer form reads GetClientSources; this writes what it really
// returns to a fixture ClientDialog.source.test.ts checks.
func TestClientSourcesFixtureForUI(t *testing.T) {
	app, _, _, _ := setUpTestInvoiceApp(t, "")
	body, err := json.MarshalIndent(mustSources(t, app), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("frontend/src/lib/testdata/client_sources.json", append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
