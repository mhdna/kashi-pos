package main

import (
	"context"
	"fmt"
	"log"
	"time"

	db "kashi-pos/db/sqlc"
)

const (
	syncWorkerInterval  = 15 * time.Second
	syncWorkerBatchSize = 200
	syncBackoffBase     = 5 * time.Second
	syncBackoffMax      = time.Hour
)

// startSyncWorker launches the background goroutine that both drains
// sync_outbox into kashi (push) and pulls master-data changes from kashi
// (see sync_pull.go). It's always started — the per-tick check for an
// empty central_api_url is what actually makes sync opt-in; an
// unconfigured install just finds nothing to do and does nothing, cheaply,
// on every tick.
func (a *App) startSyncWorker() {
	if a.store == nil {
		return
	}
	a.syncWorkerStop = make(chan struct{})
	a.syncWorkerKick = make(chan struct{}, 1)
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
				a.runSyncTick(a.ctx)
			case <-a.syncWorkerKick:
				a.runSyncTick(a.ctx)
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

// runSyncTick reads settings once per tick and, if sync is configured,
// runs the push and pull passes against the one resulting client. Reading
// settings fresh every tick (not cached at startup) means turning sync
// on/off, or changing the branch key, takes effect on the next tick
// without a restart.
func (a *App) runSyncTick(ctx context.Context) {
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		log.Printf("sync tick: load settings: %v", err)
		return
	}
	if settings.CentralApiUrl == "" {
		return
	}

	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)

	// The outbox drain keeps running even during a "sync everything" pass so
	// a live sale still reaches kashi within a tick; its writes ride the
	// same retryBusy path the resync uses. Only the pull stands down --
	// ResyncAll holds syncPullMu for its whole run anyway, so a tick pull
	// would just block.
	if err := a.runSyncOutboxOnce(ctx, client); err != nil {
		log.Printf("sync outbox: %v", err)
	}
	if err := a.pushSettings(ctx, client); err != nil {
		log.Printf("sync settings: %v", err)
	}
	a.runPendingCommandsOnce(ctx, client)
	if !a.resyncActive.Load() {
		a.runSyncPullOnce(ctx, client)
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

	for _, entry := range entries {
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

	return nil
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
	if err := a.store.MarkSyncOutboxFailed(ctx, db.MarkSyncOutboxFailedParams{
		LastError: cause.Error(),
		ID:        entry.ID,
	}); err != nil {
		log.Printf("sync outbox entry %d: mark failed: %v", entry.ID, err)
	}
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
	case "attendance_events":
		return "/branch/attendance_events", true
	case "attendance_change":
		return "/branch/attendance_changes", true
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
	if err := a.store.MarkSyncOutboxAttemptFailed(ctx, db.MarkSyncOutboxAttemptFailedParams{
		LastError:     postErr.Error(),
		NextAttemptAt: nextAttemptAt,
		ID:            entry.ID,
	}); err != nil {
		log.Printf("sync outbox entry %d: mark retry: %v", entry.ID, err)
	}
}
