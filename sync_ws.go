package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsBackoffBase = 2 * time.Second
	wsBackoffMax  = 60 * time.Second
)

// wsMessage mirrors kashi's branchWSMessage (api/branch_ws.go) — a bare
// delta pointer, never a payload. Receiving one always means "go re-fetch
// this entity," which is exactly runSyncPullOnce's job for a single
// entity — so a live push and a periodic poll can never disagree about
// how to apply a change, only about how quickly they notice one.
type wsMessage struct {
	Type      string `json:"type"`
	CommandID int64  `json:"command_id,omitempty"`
}

var wsMessageEntity = map[string]string{
	"currency_updated":        "currencies",
	"cashbox_account_updated": "cashbox_accounts",
	"products_updated":        "products",
	"client_updated":          "clients",
	"salesperson_updated":     "salespersons",
	"target_updated":          "branch_targets",
}

// startWSClient runs the reconnect loop for kashi's push channel. It's a
// separate goroutine from the poll-based sync ticker (sync_worker.go) —
// losing the WebSocket doesn't affect polling at all, it just means
// changes are noticed on the next tick instead of immediately.
func (a *App) startWSClient() {
	if a.store == nil {
		return
	}
	a.wsClientStop = make(chan struct{})
	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		a.runWSClientLoop()
	}()
}

// See stopSyncWorker's comment on why this never nils the field back out.
func (a *App) stopWSClient() {
	if a.wsClientStop != nil {
		close(a.wsClientStop)
	}
}

func (a *App) runWSClientLoop() {
	backoff := wsBackoffBase
	for {
		if !a.sleepOrStop(0) {
			return
		}

		settings, err := a.store.GetSettings(a.ctx)
		if err != nil {
			log.Printf("ws client: load settings: %v", err)
			if !a.sleepOrStop(backoff) {
				return
			}
			continue
		}
		if settings.CentralApiUrl == "" {
			// Not configured — no point spinning a tight reconnect loop.
			if !a.sleepOrStop(wsBackoffMax) {
				return
			}
			continue
		}

		client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
		conn, err := client.DialWS(a.ctx)
		if err != nil {
			log.Printf("ws client: connect failed: %v (retrying in %s)", err, backoff)
			if !a.sleepOrStop(backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		// Connected — a session that fails now is a fresh problem, not a
		// continuation of whatever caused earlier failures, so start
		// escalating from the base again next time.
		backoff = wsBackoffBase

		if err := a.runWSSession(client, conn); err != nil {
			log.Printf("ws client: session ended: %v (reconnecting in %s)", err, backoff)
			if !a.sleepOrStop(backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		// Only returns nil via a.wsClientStop firing mid-read — shutdown.
		return
	}
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > wsBackoffMax {
		return wsBackoffMax
	}
	return d
}

func (a *App) sleepOrStop(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-a.wsClientStop:
			return false
		default:
			return true
		}
	}
	select {
	case <-a.wsClientStop:
		return false
	case <-time.After(d):
		return true
	}
}

// runWSSession owns one connection's lifetime: catch up on every pulled
// entity once (so a branch that's been offline a while doesn't wait for a
// push it may never get for something that already changed while it was
// gone), then read live deltas until the connection drops or shutdown
// fires.
func (a *App) runWSSession(client *centralClient, conn *websocket.Conn) error {
	defer conn.Close()

	a.runSyncPullOnce(a.ctx, client)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-a.wsClientStop:
			conn.Close()
		case <-done:
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("ws client: bad message: %v", err)
			continue
		}
		if msg.Type == "command" {
			// Deliberately re-fetches *all* pending commands, not just
			// CommandID, mirroring the "go re-fetch" pattern everywhere
			// else in this file -- a redundant or out-of-order push just
			// means an extra fetch, never a correctness problem, and it
			// naturally sweeps up anything else still pending.
			a.runPendingCommandsOnce(a.ctx, client)
			continue
		}
		entity, ok := wsMessageEntity[msg.Type]
		if !ok {
			log.Printf("ws client: unknown message type %q", msg.Type)
			continue
		}
		if err := a.pullEntityOnce(a.ctx, client, entity); err != nil {
			log.Printf("ws client: apply %s: %v", entity, err)
		}
	}
}
