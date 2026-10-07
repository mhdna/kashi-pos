package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsProbeServer is a stand-in for kashi's /branch/ws: it upgrades the
// socket, counts connections, and otherwise answers every request with an
// empty JSON body so the session's connect-time catch-up pull is a no-op.
type wsProbeServer struct {
	*httptest.Server
	conns   int32
	upgrade websocket.Upgrader
}

func newWSProbeServer(t *testing.T) *wsProbeServer {
	t.Helper()
	p := &wsProbeServer{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			c, err := p.upgrade.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			atomic.AddInt32(&p.conns, 1)
			// Hold the socket open until the client closes it.
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *wsProbeServer) connections() int32 { return atomic.LoadInt32(&p.conns) }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// setCentralURL writes just the central_api_url on the app's settings row,
// inserting a minimal row first if none exists.
func setCentralURL(t *testing.T, app *App, url string) {
	t.Helper()
	if _, err := app.store.GetSettings(app.ctx); err != nil {
		if _, err := app.conn.ExecContext(app.ctx,
			`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id, branch_code)
			 VALUES ('Test','k','','80mm','','',80,297,'system',0,'half_up',0,0,'CN')`); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.store.UpdateCentralAPIURL(app.ctx, url); err != nil {
		t.Fatal(err)
	}
}

func TestWSClient_ReconnectsWhenCentralURLChanges(t *testing.T) {
	serverA := newWSProbeServer(t)
	serverB := newWSProbeServer(t)

	app := newVisitorTestApp(t)
	setCentralURL(t, app, serverA.URL)

	app.startWSClient()
	t.Cleanup(func() {
		app.stopWSClient()
		app.backgroundWG.Wait()
	})

	waitFor(t, "serverA connection", func() bool { return serverA.connections() >= 1 })

	// Point the branch at a different kashi server. UpdateCentralAPIURL
	// persists it and kicks the WS client; the live session bound to
	// serverA must drop and redial serverB without a restart.
	if err := app.UpdateCentralAPIURL(serverB.URL); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "serverB connection after URL change", func() bool { return serverB.connections() >= 1 })
}

func TestWSSession_EndsOnConfigRecheck(t *testing.T) {
	// Shrink the backstop ticker so the no-kick path is testable.
	orig := wsConfigRecheckInterval
	wsConfigRecheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { wsConfigRecheckInterval = orig })

	server := newWSProbeServer(t)
	app := newVisitorTestApp(t)
	app.wsClientStop = make(chan struct{})
	app.wsClientKick = make(chan struct{}, 1)
	setCentralURL(t, app, server.URL)

	client := newCentralClient(server.URL, "CN", "k")
	conn, err := client.DialWS(app.ctx)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- app.runWSSession(client, conn) }()

	waitFor(t, "session established", func() bool { return server.connections() >= 1 })

	// Change the bound key directly in the DB (no kick): the recheck ticker
	// alone must notice and end the session.
	if _, err := app.conn.ExecContext(app.ctx, `UPDATE settings SET api_key = 'rotated'`); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-result:
		if !errors.Is(err, errWSReconnect) {
			t.Fatalf("runWSSession returned %v; want errWSReconnect", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWSSession did not end after the bound config changed")
	}
}

// kashi's database-driven change feed: "sync_changed" re-fetches exactly the
// named entities (skipping any this build doesn't know), "pull_all" catches
// up on everything after a gap in the feed.
func TestHandleWSMessage_SyncChangedAndPullAll(t *testing.T) {
	var mu sync.Mutex
	var fetched []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entity := r.URL.Query().Get("entity")
		mu.Lock()
		fetched = append(fetched, entity)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"entity":%q,"items":[],"server_time":"2026-10-04T00:00:00Z"}`, entity)
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")
	// handle is one push plus the worker pass it queues.
	handle := func(msg string) {
		app.handleWSMessage(client, []byte(msg))
		app.runQueuedWSWork()
	}
	take := func() []string {
		mu.Lock()
		defer mu.Unlock()
		got := fetched
		fetched = nil
		return got
	}

	handle(`{"type":"sync_changed","entities":["price_lists","entity_from_the_future","coupons"]}`)
	if got := take(); !slices.Equal(got, []string{"price_lists", "coupons"}) {
		t.Fatalf("sync_changed fetched %v, want [price_lists coupons]", got)
	}

	// Before kashi's hello says it supports Quick Access, that entity is
	// skipped: an older kashi would answer it with an error.
	withoutQuickAccess := slices.DeleteFunc(slices.Clone(pullEntities), func(e string) bool { return e == "quick_access" })
	handle(`{"type":"pull_all"}`)
	if got := take(); !slices.Equal(got, withoutQuickAccess) {
		t.Fatalf("pull_all (no hello) fetched %v, want every pull entity but quick_access", got)
	}

	handle(`{"type":"hello","features":["sync_changed","quick_access"]}`)
	handle(`{"type":"pull_all"}`)
	if got := take(); !slices.Equal(got, pullEntities) {
		t.Fatalf("pull_all fetched %v, want every pull entity", got)
	}

	// Pushes that pile up before the worker gets to them cost one fetch per
	// entity, not one per push.
	app.handleWSMessage(client, []byte(`{"type":"sync_changed","entities":["clients"]}`))
	app.handleWSMessage(client, []byte(`{"type":"client_updated"}`))
	app.handleWSMessage(client, []byte(`{"type":"sync_changed","entities":["coupons","clients"]}`))
	app.runQueuedWSWork()
	if got := take(); !slices.Equal(got, []string{"clients", "coupons"}) {
		t.Fatalf("three queued pushes fetched %v, want [clients coupons]", got)
	}
}

// A pull that has to wait for syncPullMu -- "Sync everything" holds it for
// its whole run -- must not stall the session: kashi drops a till that
// stops answering its pings, and shows its branch offline.
func TestWSSession_KeepsAnsweringPingsWhilePullWaits(t *testing.T) {
	// kashi's heartbeat (api/ws_conn.go), scaled down: ping every 50ms, and
	// drop the till when no pong comes back within 300ms.
	const pingEvery, pongWait = 50 * time.Millisecond, 300 * time.Millisecond

	var drops, clientPulls atomic.Int32
	conns := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			entity := r.URL.Query().Get("entity")
			if entity == "clients" {
				clientPulls.Add(1)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"entity":%q,"items":[],"server_time":"2026-10-07T00:00:00Z"}`, entity)
			return
		}
		var upgrader websocket.Upgrader
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		c.SetReadDeadline(time.Now().Add(pongWait))
		c.SetPongHandler(func(string) error {
			c.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(pingEvery)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if c.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)) != nil {
						return
					}
				}
			}
		}()
		select {
		case conns <- c:
		default:
		}
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				drops.Add(1)
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	app.startWSClient()
	t.Cleanup(func() {
		app.stopWSClient()
		app.backgroundWG.Wait()
	})

	var c *websocket.Conn
	select {
	case c = <-conns:
	case <-time.After(3 * time.Second):
		t.Fatal("the till never connected")
	}
	// The connect-time catch-up pulls clients once.
	waitFor(t, "catch-up pull", func() bool { return clientPulls.Load() >= 1 })

	// Hold the lock the way a resync does, push a few changes, and give
	// kashi several pong-waits to give up on the till. Asserting only after
	// unlocking keeps a failure from leaving the worker stuck on the lock.
	app.syncPullMu.Lock()
	pullsBefore := clientPulls.Load()
	var pushErr error
	for i := 0; i < 3 && pushErr == nil; i++ {
		pushErr = c.WriteJSON(wsMessage{Type: "sync_changed", Entities: []string{"clients"}})
	}
	time.Sleep(5 * pongWait)
	dropsWhileHeld, pullsWhileHeld := drops.Load(), clientPulls.Load()
	app.syncPullMu.Unlock()

	if pushErr != nil {
		t.Fatal(pushErr)
	}
	if dropsWhileHeld != 0 {
		t.Fatalf("kashi dropped the till %d time(s) while a pushed pull waited on syncPullMu", dropsWhileHeld)
	}
	if pullsWhileHeld != pullsBefore {
		t.Fatalf("clients pulled while syncPullMu was held (%d -> %d)", pullsBefore, pullsWhileHeld)
	}
	// Once the lock frees, the pushed change still lands.
	waitFor(t, "pushed pull after the lock frees", func() bool { return clientPulls.Load() > pullsBefore })
}
