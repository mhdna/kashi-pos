package main

import (
	"encoding/json"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsBackoffBase = 2 * time.Second
	wsBackoffMax  = 60 * time.Second
)

// wsPingInterval/wsPongWait/wsWriteWait mirror kashi's own heartbeat
// (api/ws_conn.go) -- ping the server well inside how long we'll wait for a
// pong, so a connection that's actually dead (kashi restarted, or something
// in between silently dropped the TCP connection) is detected within
// roughly wsPongWait instead of only surfacing whenever the OS eventually
// reaps it. This channel is push-only from kashi's side and can otherwise
// sit idle a long time between real messages.
const (
	wsPingInterval = 30 * time.Second
	wsPongWait     = 60 * time.Second
	wsWriteWait    = 10 * time.Second
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
	// Entities is set on "sync_changed": the pull entities kashi's
	// database reported changes to (kashi's api/sync_notify.go).
	Entities []string `json:"entities,omitempty"`
	// Features is set on "hello", kashi's first message of a session.
	Features []string `json:"features,omitempty"`
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

// startWSClient runs the reconnect loop for kashi's push channel, and the
// worker that does what the pushes ask for (runWSWorker). Both are
// separate goroutines from the poll-based sync ticker (sync_worker.go) —
// losing the WebSocket doesn't affect polling at all, it just means
// changes are noticed on the next tick instead of immediately.
func (a *App) startWSClient() {
	if a.store == nil {
		return
	}
	a.wsClientStop = make(chan struct{})
	a.wsClientKick = make(chan struct{}, 1)
	a.wsWorkKick = make(chan struct{}, 1)
	a.backgroundWG.Add(2)
	go func() {
		defer a.backgroundWG.Done()
		a.runWSClientLoop()
	}()
	go func() {
		defer a.backgroundWG.Done()
		a.runWSWorker()
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
//
// The session itself only reads, pings and queues: the catch-up, and every
// pull or command a push asks for, run on runWSWorker. A pull can wait
// minutes for syncPullMu ("Sync everything" holds it for its whole run),
// and a session stuck behind one stops answering kashi's pings -- gorilla
// only answers them while something reads -- so kashi drops the till as
// dead and shows its branch offline exactly while it's busiest.
func (a *App) runWSSession(client *centralClient, conn *websocket.Conn) error {
	defer conn.Close()
	// Without a live session nothing is being pushed: back to full polling.
	defer a.wsPushesAll.Store(false)
	defer a.kashiQuickAccess.Store(false)
	// kashi may have restarted while we were away; report settings afresh.
	a.settingsReport.forget()

	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	a.queueWSWork(wsWork{client: client, pullAll: true})

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

	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()

	for {
		select {
		case <-a.wsClientStop:
			return teardown(nil)
		case <-a.wsClientKick:
			return teardown(errWSReconnect)
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return teardown(err)
			}
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

// handleWSMessage applies one push frame. It runs on the session loop, so
// it never waits: hello and expense_images_uploaded are applied here, and
// everything that needs a request is queued for runWSWorker. Every pull is
// a "go re-fetch" trigger, never a payload apply, so a duplicate or
// out-of-order frame is only ever a redundant fetch.
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
		a.queueWSWork(wsWork{client: client, commands: true})
		return
	}
	if msg.Type == "app_update" {
		// An admin changed which release this till should run.
		a.kickUpdater()
		return
	}
	if msg.Type == "expense_images_uploaded" {
		a.emitExpenseImagesUploaded(msg.ClientRef)
		return
	}
	if msg.Type == "loan_images_uploaded" {
		a.emitLoanImagesUploaded(msg.ClientRef)
		return
	}
	if msg.Type == "hello" {
		// A kashi that pushes every change lets the pull loop slow down to a
		// backstop. An older kashi sends no hello, and its tills keep the
		// 15s poll. runWSSession clears this when the session ends.
		a.wsPushesAll.Store(slices.Contains(msg.Features, "sync_changed"))
		a.kashiQuickAccess.Store(slices.Contains(msg.Features, "quick_access"))
		return
	}
	if msg.Type == "pull_all" {
		// kashi's change feed had a gap; catch up on everything.
		a.queueWSWork(wsWork{client: client, pullAll: true})
		return
	}
	if msg.Type == "sync_changed" {
		a.queueWSWork(wsWork{client: client, entities: msg.Entities})
		return
	}
	entity, ok := wsMessageEntity[msg.Type]
	if !ok {
		log.Printf("ws client: unknown message type %q", msg.Type)
		return
	}
	a.queueWSWork(wsWork{client: client, entities: []string{entity}})
}

// wsWork is what pushes have asked runWSWorker to do: run pending commands,
// catch up on every entity, re-fetch some. client is the session that
// asked.
type wsWork struct {
	client   *centralClient
	commands bool
	pullAll  bool
	entities []string
}

// queueWSWork adds a push's work to the worker's next pass and wakes it,
// without waiting. Pushes that arrive while the worker is busy merge into
// that one pass -- the tenth branch_stock change during a resync is the
// same fetch as the first -- and the pass uses the latest session's client.
func (a *App) queueWSWork(w wsWork) {
	a.wsWorkMu.Lock()
	a.wsWork.client = w.client
	a.wsWork.commands = a.wsWork.commands || w.commands
	a.wsWork.pullAll = a.wsWork.pullAll || w.pullAll
	for _, entity := range w.entities {
		if !slices.Contains(a.wsWork.entities, entity) {
			a.wsWork.entities = append(a.wsWork.entities, entity)
		}
	}
	a.wsWorkMu.Unlock()
	select {
	case a.wsWorkKick <- struct{}{}:
	default:
	}
}

// runWSWorker does the work pushes queue, off the session loop -- see
// runWSSession for why the session can't do it itself.
func (a *App) runWSWorker() {
	for {
		select {
		case <-a.wsClientStop:
			return
		case <-a.wsWorkKick:
			a.runQueuedWSWork()
		}
	}
}

// runQueuedWSWork takes everything queued so far and does it. Commands go
// first: they don't need syncPullMu, so pulls in the same pass that wait
// out a resync don't hold them up.
func (a *App) runQueuedWSWork() {
	a.wsWorkMu.Lock()
	w := a.wsWork
	a.wsWork = wsWork{}
	a.wsWorkMu.Unlock()

	if w.commands {
		a.runPendingCommandsOnce(a.ctx, w.client)
	}
	if w.pullAll {
		a.runSyncPullOnce(a.ctx, w.client)
	}
	// Walking pullEntities rather than w.entities skips any entity a newer
	// kashi names that this build doesn't pull yet.
	for _, entity := range pullEntities {
		if !slices.Contains(w.entities, entity) {
			continue
		}
		if err := a.pullEntityOnce(a.ctx, w.client, entity); err != nil {
			log.Printf("ws client: apply %s: %v", entity, err)
		}
	}
}
