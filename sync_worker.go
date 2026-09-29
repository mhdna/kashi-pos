package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

const (
	syncWorkerInterval       = 15 * time.Second
	syncPushBackstopInterval = 5 * time.Second
	syncWorkerBatchSize      = 200
	syncBackoffBase          = 5 * time.Second
	syncBackoffMax           = time.Hour
)

// startSyncWorker launches two background goroutines: a push loop that
// drains sync_outbox into kashi and a pull loop that pulls master-data
// changes from kashi (see sync_pull.go). They're split so an outgoing
// write never waits on an incoming catch-up pass -- a kick wakes only the
// push loop, and a press reaches kashi in one outbox drain rather than one
// full tick (outbox + settings + commands + a 9-entity sequential pull).
// Both are always started -- the per-pass check for an empty
// central_api_url is what actually makes sync opt-in; an unconfigured
// install just finds nothing to do and does nothing, cheaply.
func (a *App) startSyncWorker() {
	if a.store == nil {
		return
	}
	a.syncWorkerStop = make(chan struct{})
	a.syncWorkerKick = make(chan struct{}, 1)

	// Push loop (latency path): kick-driven, with a short ticker only as a
	// backstop for a kick that was lost to the buffer-of-1 while a drain
	// was already running.
	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		ticker := time.NewTicker(syncPushBackstopInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.syncWorkerStop:
				return
			case <-ticker.C:
				a.runPushTick(a.ctx)
			case <-a.syncWorkerKick:
				a.runPushTick(a.ctx)
			}
		}
	}()

	// Pull loop (no latency requirement): the 15s ticker alone.
	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		ticker := time.NewTicker(syncWorkerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.syncWorkerStop:
				return
			case <-ticker.C:
				a.runPullTick(a.ctx)
			}
		}
	}()
}

// kickSyncWorker wakes the sync worker immediately rather than leaving a
// freshly-queued outbox entry to wait for the next syncWorkerInterval tick
// (up to 15s of pure latency between checkout and kashi actually receiving
// it). The buffered, non-blocking send means: a kick that lands while the
// worker is already mid-tick is coalesced into the very next tick rather
// than queued up or lost outright, and calling this before startSyncWorker
// has run (or when a.store is nil, so it never ran at all) is a safe no-op
// -- a nil channel's send case in a select is simply never ready, so
// control falls straight through to default.
func (a *App) kickSyncWorker() {
	select {
	case a.syncWorkerKick <- struct{}{}:
	default:
	}
}

// syncClient reads settings fresh (not cached at startup, so turning sync
// on/off or changing the branch key takes effect on the next pass without
// a restart) and builds the central client. It returns nil when sync isn't
// configured, which every caller treats as "nothing to do".
func (a *App) syncClient(ctx context.Context, what string) *centralClient {
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		log.Printf("sync %s: load settings: %v", what, err)
		return nil
	}
	if settings.CentralApiUrl == "" {
		return nil
	}
	return newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
}

// runPushTick is the latency path: drain sync_outbox into kashi and nothing
// else. It keeps running even during a "sync everything" pass so a live
// sale still reaches kashi promptly; its writes ride the same retryBusy
// path the resync uses.
func (a *App) runPushTick(ctx context.Context) {
	client := a.syncClient(ctx, "push")
	if client == nil {
		return
	}
	if err := a.runSyncOutboxOnce(ctx, client); err != nil {
		log.Printf("sync outbox: %v", err)
	}
}

// runSyncTick runs a push pass then a pull pass back to back. The live
// worker no longer calls this -- the push and pull loops run
// independently -- but it stays as the one-shot "do a full sync now"
// entry point for tests and for any caller that wants both passes in
// sequence.
func (a *App) runSyncTick(ctx context.Context) {
	a.runPushTick(ctx)
	a.runPullTick(ctx)
}

// runPullTick is everything with no latency requirement: push settings, run
// pending commands, and pull master-data changes. The pull stands down
// during a resync -- ResyncAll holds syncPullMu for its whole run anyway,
// so a tick pull would just block.
func (a *App) runPullTick(ctx context.Context) {
	client := a.syncClient(ctx, "pull")
	if client == nil {
		return
	}
	if err := a.pushSettings(ctx, client); err != nil {
		log.Printf("sync settings: %v", err)
	}
	a.runPendingCommandsOnce(ctx, client)
	a.runShiftReceiptUploadsOnce(ctx, client)
	if !a.resyncActive.Load() {
		a.runSyncPullOnce(ctx, client)
		a.maybeRunFullCheck(ctx, client)
	}
}

// stopSyncWorker closes the stop channel but deliberately never nils the
// field back out: start/stop each happen exactly once per app lifetime, so
// there's no restart case to support, and niling it here would race with
// the worker goroutine's own read of a.syncWorkerStop (a read that lands
// after the nil-assignment sees nil, and <-nil blocks forever instead of
// firing like a closed channel does).
func (a *App) stopSyncWorker() {
	if a.syncWorkerStop != nil {
		close(a.syncWorkerStop)
	}
}

// runSyncOutboxOnce drains up to syncWorkerBatchSize due entries in one pass.
// A row ends a pass in exactly one of three states: 'synced' (kashi 2xx'd
// it, stamped with which server), still 'pending' (a transient failure, on
// backoff), or 'failed' (kashi rejected it outright, or it names an entity
// kashi has no route for -- either way retrying the identical bytes forever
// is pointless). Any transition emits sync_state_changed so the UI refetches.
func (a *App) runSyncOutboxOnce(ctx context.Context, client *centralClient) error {
	entries, err := a.store.ListDueSyncOutboxEntries(ctx, syncWorkerBatchSize)
	if err != nil {
		return fmt.Errorf("list due outbox entries: %w", err)
	}

	changed := false
	defer func() {
		if changed {
			a.emitSyncStateChanged()
		}
	}()

	// Coalesce all due visitor-counter presses into a single batched POST.
	// The write path queues one row per press so the local log stays exact;
	// draining them one HTTP call each turns a rapid ▲▲▲▲▲ into a serial
	// chain of round trips (and, mid-burst, makes every press after the
	// first wait out the previous one). Grouping here also merges presses
	// that arrived as separate AdjustVisitorCountToday calls.
	var visitorEntries, clientEntries, rest []db.SyncOutbox
	for _, entry := range entries {
		switch entry.EntityType {
		case "visitor_event":
			visitorEntries = append(visitorEntries, entry)
		case "client":
			clientEntries = append(clientEntries, entry)
		default:
			rest = append(rest, entry)
		}
	}
	if a.drainVisitorEventBatch(ctx, client, visitorEntries) {
		changed = true
	}

	// A lone client row is the live checkout path (a buyer queued right
	// before their invoice): send it the plain per-row way. Two or more is
	// a bulk backfill -- one POST for the lot instead of a serial round
	// trip each (see drainClientBatch). They sort last (priority 2), so
	// appending keeps the drain order. Both routes answer with the
	// canonical uid kashi maps the client to (see adoptCanonicalClientUID).
	batchClients := len(clientEntries) >= 2
	if !batchClients {
		rest = append(rest, clientEntries...)
	}

	sent := 0
	for _, entry := range rest {
		// A row queued after this pass fetched its list (a fresh sale)
		// can't be in `rest`; without this a long pass makes it wait out
		// every remaining row. Bail so the pending kick starts a new
		// pass, whose fetch puts it first. Always finish one row so a
		// stale kick can't stall the drain.
		if sent > 0 && a.pushPreempted() {
			return nil
		}
		sent++

		// transfer_confirm has no fixed path -- it targets one specific
		// central transfer id, parsed back out of the outbox row's own
		// synthesized client_ref (see queueBranchTransferConfirmSync).
		if entry.EntityType == "transfer_confirm" {
			centralID, parseErr := parseTransferConfirmClientRef(entry.ClientRef)
			if parseErr != nil {
				log.Printf("sync outbox entry %d: %v, marking failed", entry.ID, parseErr)
				a.markSyncOutboxFailed(ctx, entry, parseErr)
				changed = true
				continue
			}
			path := fmt.Sprintf("/branch/transfer_requests/%d/confirm", centralID)
			if err := client.PostInvoice(ctx, path, []byte(entry.PayloadJson)); err != nil {
				a.recordSyncOutboxError(ctx, entry, err)
				changed = true
				continue
			}
			if err := retryBusy(func() error {
				return a.store.MarkSyncOutboxSynced(ctx, db.MarkSyncOutboxSyncedParams{
					SyncedTo: client.baseURL,
					ID:       entry.ID,
				})
			}); err != nil {
				log.Printf("sync outbox entry %d: mark synced: %v", entry.ID, err)
			}
			changed = true
			continue
		}

		path, ok := branchInvoicePath(entry.EntityType)
		if !ok {
			// A payload shape kashi has no route for is never going to start
			// working on retry. Park it in 'failed' so it stops being
			// re-selected every tick and occupying a drain-batch slot.
			log.Printf("sync outbox entry %d: unknown entity_type %q, marking failed", entry.ID, entry.EntityType)
			a.markSyncOutboxFailed(ctx, entry, fmt.Errorf("unknown entity_type %q", entry.EntityType))
			changed = true
			continue
		}

		// Expenses need kashi's response body (the one-time receipt-upload
		// token) -- every other entity's response is discarded, since
		// kashi's idempotency handling already makes any 2xx equally a
		// success for them.
		if entry.EntityType == "expense" {
			resp, err := client.PostExpense(ctx, path, []byte(entry.PayloadJson))
			if err != nil {
				a.recordSyncOutboxError(ctx, entry, err)
				changed = true
				continue
			}
			if resp.UploadToken != "" {
				a.emitExpenseUploadReady(entry.ClientRef, client.UploadURL(resp.UploadToken))
			}
		} else if entry.EntityType == "transfer_request" {
			// Same shape as "expense" above: the response body carries the
			// one-time photo-upload token this transfer's QR points at.
			resp, err := client.PostTransferRequest(ctx, path, []byte(entry.PayloadJson))
			if err != nil {
				a.recordSyncOutboxError(ctx, entry, err)
				changed = true
				continue
			}
			if resp.UploadToken != "" {
				a.emitTransferUploadReady(entry.ClientRef, client.TransferUploadURL(resp.UploadToken))
			}
		} else if entry.EntityType == "client" {
			if err := a.pushOneClient(ctx, client, path, entry); err != nil {
				a.recordSyncOutboxError(ctx, entry, err)
				changed = true
				continue
			}
		} else if err := client.PostInvoice(ctx, path, []byte(entry.PayloadJson)); err != nil {
			a.recordSyncOutboxError(ctx, entry, err)
			changed = true
			continue
		}

		if err := retryBusy(func() error {
			return a.store.MarkSyncOutboxSynced(ctx, db.MarkSyncOutboxSyncedParams{
				SyncedTo: client.baseURL,
				ID:       entry.ID,
			})
		}); err != nil {
			log.Printf("sync outbox entry %d: mark synced: %v", entry.ID, err)
		}
		changed = true
	}

	if batchClients && !(sent > 0 && a.pushPreempted()) {
		if a.drainClientBatch(ctx, client, clientEntries) {
			changed = true
		}
	}

	// A full page means there is probably more queued behind it (a bulk
	// backfill); keep draining without waiting for the 5s backstop tick.
	// The next pass re-fetches from the top, so a sale queued meanwhile
	// still goes first.
	if len(entries) == syncWorkerBatchSize && changed {
		a.kickSyncWorker()
	}

	return nil
}

// pushPreempted reports whether a kick is waiting: something was queued
// after the running push pass began. It only peeks -- the push loop
// consumes the kick when it starts the next pass. len of a nil channel is 0,
// so this is false before startSyncWorker has run.
func (a *App) pushPreempted() bool {
	return len(a.syncWorkerKick) > 0
}

// drainClientBatch POSTs the due client rows as one /branch/clients_batch
// call and settles each row from kashi's per-client result: ok -> synced,
// invalid -> failed (identical bytes never get accepted), error or no
// result -> back on the backoff path. It returns whether any row changed
// state. If kashi has no such route (an older deployment) the rows fall
// back to the one-at-a-time POST so a mixed rollout never strands them.
func (a *App) drainClientBatch(ctx context.Context, client *centralClient, entries []db.SyncOutbox) bool {
	changed := false
	payload := branchClientsBatchPayload{}
	send := make([]db.SyncOutbox, 0, len(entries))
	for _, entry := range entries {
		var c branchClientPayload
		if err := json.Unmarshal([]byte(entry.PayloadJson), &c); err != nil {
			log.Printf("sync outbox entry %d: bad client payload, marking failed: %v", entry.ID, err)
			a.markSyncOutboxFailed(ctx, entry, err)
			changed = true
			continue
		}
		payload.Clients = append(payload.Clients, c)
		send = append(send, entry)
	}
	if len(send) == 0 {
		return changed
	}

	results, err := client.PostClientsBatch(ctx, payload)
	if err != nil {
		var perm *permanentError
		if errors.As(err, &perm) && (perm.status == http.StatusNotFound || perm.status == http.StatusMethodNotAllowed) {
			log.Printf("sync clients batch: kashi has no clients_batch route (%v), sending one by one", err)
			return a.drainClientsOneByOne(ctx, client, send) || changed
		}
		for _, entry := range send {
			// A 4xx on the whole envelope must not park every row as
			// 'failed': the fault is the batch, not each client. Retry.
			a.markSyncOutboxRetry(ctx, entry, err)
		}
		return true
	}

	byID := make(map[int64]branchClientBatchResult, len(results))
	for _, r := range results {
		byID[r.BranchClientID] = r
	}
	for i, entry := range send {
		res, ok := byID[payload.Clients[i].BranchClientID]
		switch {
		case !ok:
			a.markSyncOutboxRetry(ctx, entry, fmt.Errorf("clients batch: no result for client %d", payload.Clients[i].BranchClientID))
		case res.Status == "ok":
			a.adoptPushedClientUID(ctx, entry, res.UID)
			a.markOutboxSynced(ctx, client, entry)
		case res.Status == "invalid":
			log.Printf("sync outbox entry %d: kashi rejected client (permanent, marking failed): %s", entry.ID, res.Error)
			a.markSyncOutboxFailed(ctx, entry, errors.New(res.Error))
		default:
			a.markSyncOutboxRetry(ctx, entry, fmt.Errorf("clients batch: %s", res.Error))
		}
	}
	return true
}

// drainClientsOneByOne is the pre-batch path for client rows, kept as the
// fallback for a kashi without /branch/clients_batch.
func (a *App) drainClientsOneByOne(ctx context.Context, client *centralClient, entries []db.SyncOutbox) bool {
	path, _ := branchInvoicePath("client")
	for i, entry := range entries {
		if i > 0 && a.pushPreempted() {
			break
		}
		if err := a.pushOneClient(ctx, client, path, entry); err != nil {
			a.recordSyncOutboxError(ctx, entry, err)
			continue
		}
		a.markOutboxSynced(ctx, client, entry)
	}
	return true
}

// pushOneClient POSTs one client outbox row and applies the canonical uid
// kashi answers with.
func (a *App) pushOneClient(ctx context.Context, client *centralClient, path string, entry db.SyncOutbox) error {
	uid, err := client.PostClient(ctx, path, []byte(entry.PayloadJson))
	if err != nil {
		return err
	}
	a.adoptPushedClientUID(ctx, entry, uid)
	return nil
}

// adoptPushedClientUID applies kashi's canonical uid for a pushed client
// row. A failure here is logged, not retried: the push itself landed, and
// the next pull or push converges the uid anyway.
func (a *App) adoptPushedClientUID(ctx context.Context, entry db.SyncOutbox, uid string) {
	if uid == "" {
		return
	}
	var p branchClientPayload
	if err := json.Unmarshal([]byte(entry.PayloadJson), &p); err != nil {
		return
	}
	if err := a.resyncTx(ctx, func(q *db.Queries) error {
		return adoptCanonicalClientUID(ctx, q, p.BranchClientID, uid)
	}); err != nil {
		log.Printf("sync outbox entry %d: adopt canonical client uid: %v", entry.ID, err)
	}
}

// markOutboxSynced stamps a row synced against the server it went to.
func (a *App) markOutboxSynced(ctx context.Context, client *centralClient, entry db.SyncOutbox) {
	if err := retryBusy(func() error {
		return a.store.MarkSyncOutboxSynced(ctx, db.MarkSyncOutboxSyncedParams{
			SyncedTo: client.baseURL,
			ID:       entry.ID,
		})
	}); err != nil {
		log.Printf("sync outbox entry %d: mark synced: %v", entry.ID, err)
	}
}

// drainVisitorEventBatch POSTs every due visitor_event row as one
// /branch/visitor_events_batch call. kashi dedupes each element on
// (branch_id, ref), so replaying the whole batch (a retried drain) is
// idempotent row by row. It returns whether anything changed state, so the
// caller can fold that into its sync_state_changed emit. A row whose stored
// payload won't unmarshal is parked in 'failed' and left out of the POST --
// retrying identical bad bytes is pointless.
func (a *App) drainVisitorEventBatch(ctx context.Context, client *centralClient, entries []db.SyncOutbox) bool {
	if len(entries) == 0 {
		return false
	}

	path, ok := branchInvoicePath("visitor_events")
	if !ok {
		return false
	}

	changed := false
	var send []db.SyncOutbox
	payload := branchVisitorEventsPayload{}
	for _, entry := range entries {
		var ev branchVisitorEventPayload
		if err := json.Unmarshal([]byte(entry.PayloadJson), &ev); err != nil {
			log.Printf("sync outbox entry %d: bad visitor_event payload, marking failed: %v", entry.ID, err)
			a.markSyncOutboxFailed(ctx, entry, err)
			changed = true
			continue
		}
		payload.Events = append(payload.Events, ev)
		send = append(send, entry)
	}
	if len(send) == 0 {
		return changed
	}

	body, err := json.Marshal(payload)
	if err != nil {
		// A marshal failure here is a programming error, not a transient
		// one; back every row off rather than losing them.
		for _, entry := range send {
			a.recordSyncOutboxError(ctx, entry, err)
		}
		return true
	}

	if err := client.PostInvoice(ctx, path, body); err != nil {
		for _, entry := range send {
			a.recordSyncOutboxError(ctx, entry, err)
		}
		return true
	}

	for _, entry := range send {
		if err := retryBusy(func() error {
			return a.store.MarkSyncOutboxSynced(ctx, db.MarkSyncOutboxSyncedParams{
				SyncedTo: client.baseURL,
				ID:       entry.ID,
			})
		}); err != nil {
			log.Printf("sync outbox entry %d: mark synced: %v", entry.ID, err)
		}
	}
	return true
}

// recordSyncOutboxError routes a failed push to the right terminal state: a
// permanent rejection (4xx bar 408/429, see isPermanentSyncError) goes
// straight to 'failed'; anything else -- offline, timeout, 5xx -- goes back
// on the exponential-backoff retry path.
func (a *App) recordSyncOutboxError(ctx context.Context, entry db.SyncOutbox, postErr error) {
	if isPermanentSyncError(postErr) {
		log.Printf("sync outbox entry %d: %v (permanent, marking failed)", entry.ID, postErr)
		a.markSyncOutboxFailed(ctx, entry, postErr)
		return
	}
	a.markSyncOutboxRetry(ctx, entry, postErr)
}

// markSyncOutboxFailed parks a row in the terminal 'failed' state. Only an
// explicit resync (see sync_backfill.go) re-arms it.
func (a *App) markSyncOutboxFailed(ctx context.Context, entry db.SyncOutbox, cause error) {
	if err := retryBusy(func() error {
		return a.store.MarkSyncOutboxFailed(ctx, db.MarkSyncOutboxFailedParams{
			LastError: cause.Error(),
			ID:        entry.ID,
		})
	}); err != nil {
		log.Printf("sync outbox entry %d: mark failed: %v", entry.ID, err)
	}
}

// parseTransferConfirmClientRef recovers the central transfer id a
// "transfer_confirm" outbox row targets from its own synthesized
// client_ref -- see queueBranchTransferConfirmSync.
func parseTransferConfirmClientRef(clientRef string) (int64, error) {
	const prefix = "transfer-confirm-"
	if !strings.HasPrefix(clientRef, prefix) {
		return 0, fmt.Errorf("malformed transfer_confirm client_ref %q", clientRef)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(clientRef, prefix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed transfer_confirm client_ref %q: %w", clientRef, err)
	}
	return id, nil
}

func branchInvoicePath(entityType string) (string, bool) {
	switch entityType {
	case "sales_invoice":
		return "/branch/sales_invoices", true
	case "return_invoice":
		return "/branch/return_invoices", true
	case "client":
		return "/branch/clients", true
	case "expense":
		return "/branch/expenses", true
	case "loan":
		return "/branch/loans", true
	case "invoice_settlement":
		return "/branch/invoice_settlements", true
	case "shift_close":
		return "/branch/shift_closes", true
	case "visitor_event":
		return "/branch/visitor_events", true
	case "visitor_events":
		// The batched route. No outbox row is stored with this entity_type;
		// the sync worker coalesces the singular "visitor_event" rows and
		// posts them here (see drainVisitorEventBatch).
		return "/branch/visitor_events_batch", true
	case "attendance_events":
		return "/branch/attendance_events", true
	case "attendance_change":
		return "/branch/attendance_changes", true
	case "transfer_request":
		return "/branch/transfer_requests", true
	default:
		return "", false
	}
}

// markSyncOutboxRetry schedules the next attempt with exponential backoff.
// It never gives up — an offline branch may stay offline for a long time,
// and that must not mean a sale silently stops trying to sync.
func (a *App) markSyncOutboxRetry(ctx context.Context, entry db.SyncOutbox, postErr error) {
	delay := syncBackoffBase << entry.AttemptCount
	if delay > syncBackoffMax || delay <= 0 {
		delay = syncBackoffMax
	}
	nextAttemptAt := time.Now().UTC().Add(delay).Format("2006-01-02 15:04:05")

	log.Printf("sync outbox entry %d: %v (retrying at %s)", entry.ID, postErr, nextAttemptAt)
	if err := retryBusy(func() error {
		return a.store.MarkSyncOutboxAttemptFailed(ctx, db.MarkSyncOutboxAttemptFailedParams{
			LastError:     postErr.Error(),
			NextAttemptAt: nextAttemptAt,
			ID:            entry.ID,
		})
	}); err != nil {
		log.Printf("sync outbox entry %d: mark retry: %v", entry.ID, err)
	}
}
