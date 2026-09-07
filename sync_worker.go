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
	syncWorkerBatchSize = 20
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

	if err := a.runSyncOutboxOnce(ctx, client); err != nil {
		log.Printf("sync outbox: %v", err)
	}
	if err := a.pushSettings(ctx, client); err != nil {
		log.Printf("sync settings: %v", err)
	}
	a.runPendingCommandsOnce(ctx, client)
	a.runSyncPullOnce(ctx, client)
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
func (a *App) runSyncOutboxOnce(ctx context.Context, client *centralClient) error {
	entries, err := a.store.ListDueSyncOutboxEntries(ctx, syncWorkerBatchSize)
	if err != nil {
		return fmt.Errorf("list due outbox entries: %w", err)
	}

	for _, entry := range entries {
		path, ok := branchInvoicePath(entry.EntityType)
		if !ok {
			// A payload shape kashi doesn't have a route for isn't going to
			// start working on retry — record it and move on rather than
			// spin on it forever.
			log.Printf("sync outbox entry %d: unknown entity_type %q, skipping", entry.ID, entry.EntityType)
			continue
		}

		// Expenses need kashi's response body (the one-time receipt-upload
		// token) -- every other entity's response is discarded, since
		// kashi's idempotency handling already makes any 2xx equally a
		// success for them.
		if entry.EntityType == "expense" {
			resp, err := client.PostExpense(ctx, path, []byte(entry.PayloadJson))
			if err != nil {
				a.markSyncOutboxRetry(ctx, entry, err)
				continue
			}
			if resp.UploadToken != "" {
				a.emitExpenseUploadReady(entry.ClientRef, client.UploadURL(resp.UploadToken))
			}
		} else if err := client.PostInvoice(ctx, path, []byte(entry.PayloadJson)); err != nil {
			a.markSyncOutboxRetry(ctx, entry, err)
			continue
		}

		if err := a.store.MarkSyncOutboxSynced(ctx, entry.ID); err != nil {
			log.Printf("sync outbox entry %d: mark synced: %v", entry.ID, err)
		}
	}

	return nil
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
