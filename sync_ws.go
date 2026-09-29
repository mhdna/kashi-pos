package main

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsBackoffBase = 2 * time.Second
	wsBackoffMax  = 60 * time.Second
)

// wsConfigRecheckInterval is how often a live session re-reads settings to
// notice the central URL / branch code / key changing under it. A
// kickWSClient() from the save path makes the common case instant; this
// ticker is the backstop for changes that arrive another way (a remote
// update_settings command, a direct DB edit). It's a var, not a const, so
// tests can shrink it.
var wsConfigRecheckInterval = 10 * time.Second

// errWSReconnect is returned by runWSSession when the session was torn down
// on purpose because its central config went stale -- runWSClientLoop
// treats it as "dial again now", with no backoff, since it's a
// reconfiguration and not a failure.
var errWSReconnect = errors.New("ws client: central config changed, reconnecting")

// wsMessage mirrors kashi's branchWSMessage (api/branch_ws.go) — a bare
// delta pointer, never a payload. Receiving one always means "go re-fetch
// this entity," which is exactly runSyncPullOnce's job for a single
// entity — so a live push and a periodic poll can never disagree about
// how to apply a change, only about how quickly they notice one.
type wsMessage struct {
	Type      string `json:"type"`
	CommandID int64  `json:"command_id,omitempty"`
	// ClientRef is only set for "expense_images_uploaded" -- unlike every
	// other pushed message, that one needs to say *which* expense, not
	// just "go re-fetch this kind of thing".
	ClientRef string `json:"client_ref,omitempty"`
}

var wsMessageEntity = map[string]string{
	"currency_updated":                   "currencies",
	"cashbox_account_updated":            "cashbox_accounts",
	"products_updated":                   "products",
	"client_updated":                     "clients",
	"salesperson_updated":                "salespersons",
	"branch_user_updated":                "branch_users",
	"target_updated":                     "branch_targets",
	"expense_categories_updated":         "expense_categories",
	"loan_categories_updated":            "loan_categories",
	"incoming_transfer_requests_updated": "incoming_transfer_requests",
	"transfer_requests_updated":          "transfer_requests",
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
	a.wsClientKick = make(chan struct{}, 1)
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

// kickWSClient tells the WS client that the central config changed, so a
// live session dialed at the old URL/key should drop and reconnect at the
// new one immediately rather than waiting for wsConfigRecheckInterval.
// Same nil-safe, non-blocking, coalescing shape as kickSyncWorker: a kick
// before startWSClient (or with a.store nil) is a no-op, and one that
// lands while a session is mid-teardown is folded into the next.
func (a *App) kickWSClient() {
	select {
	case a.wsClientKick <- struct{}{}:
	default:
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
			if errors.Is(err, errWSReconnect) {
				// Deliberate teardown after a config change -- redial now
				// with the new settings, from the base backoff.
				log.Printf("ws client: central config changed, reconnecting")
				backoff = wsBackoffBase
				continue
			}
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

	// The central config this session is bound to. If the URL / branch code
	// / key changes under us -- Settings page save, remote update_settings
	// command -- a still-healthy socket to the *old* server would otherwise
	// keep this session alive forever and the change wouldn't land until a
	// process restart. Both the recheck ticker below and kickWSClient()
	// exist to end the session so runWSClientLoop redials the new target.
	boundURL, boundCode, boundKey := client.baseURL, client.branchCode, client.apiKey

	type wsFrame struct {
		data []byte
		err  error
	}
	frames := make(chan wsFrame, 1)
	readerExited := make(chan struct{})
	abort := make(chan struct{})
	go func() {
		defer close(readerExited)
		for {
			_, data, err := conn.ReadMessage()
			select {
			case frames <- wsFrame{data: data, err: err}:
			case <-abort:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	// teardown unblocks the reader (closing the conn makes ReadMessage
	// return), lets it abandon a pending send, waits for it to exit so it
	// can't outlive the session, and returns ret.
	teardown := func(ret error) error {
		conn.Close()
		close(abort)
		<-readerExited
		return ret
	}

	recheck := time.NewTicker(wsConfigRecheckInterval)
	defer recheck.Stop()

	for {
		select {
		case <-a.wsClientStop:
			return teardown(nil)
		case <-a.wsClientKick:
			return teardown(errWSReconnect)
		case <-recheck.C:
			s, err := a.store.GetSettings(a.ctx)
			if err != nil {
				continue // transient read error -- try again next tick
			}
			if s.CentralApiUrl != boundURL || s.BranchCode != boundCode || s.ApiKey != boundKey {
				return teardown(errWSReconnect)
			}
		case f := <-frames:
			if f.err != nil {
				return teardown(f.err)
			}
			a.handleWSMessage(client, f.data)
		}
	}
}

// handleWSMessage applies one push frame. Every branch is a "go re-fetch"
// trigger, never a payload apply, so a duplicate or out-of-order frame is
// only ever a redundant fetch.
func (a *App) handleWSMessage(client *centralClient, data []byte) {
	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("ws client: bad message: %v", err)
		return
	}
	if msg.Type == "command" {
		// Deliberately re-fetches *all* pending commands, not just
		// CommandID -- a redundant or out-of-order push just means an extra
		// fetch, and it naturally sweeps up anything else still pending.
		a.runPendingCommandsOnce(a.ctx, client)
		return
	}
	if msg.Type == "expense_images_uploaded" {
		a.emitExpenseImagesUploaded(msg.ClientRef)
		return
	}
	entity, ok := wsMessageEntity[msg.Type]
	if !ok {
		log.Printf("ws client: unknown message type %q", msg.Type)
		return
	}
	if err := a.pullEntityOnce(a.ctx, client, entity); err != nil {
		log.Printf("ws client: apply %s: %v", entity, err)
	}
}
