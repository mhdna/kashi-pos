package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// shiftReceiptUploadBatch bounds how many buffered photos one pass forwards
// to kashi. In practice there's almost never more than one outstanding (one
// per shift close), but a branch that was offline for days could have a
// small backlog.
const shiftReceiptUploadBatch = 20

// runShiftReceiptUploadsOnce forwards every close-shift receipt photo that's
// been captured on this POS but not yet confirmed by kashi. It's called from
// runPullTick, so it runs on the 15s cadence and, being entirely driven by
// shift_receipt_uploads rows, simply resumes after an app restart or a
// multi-day outage -- there is no terminal "give up" state.
func (a *App) runShiftReceiptUploadsOnce(ctx context.Context, client *centralClient) {
	rows, err := a.store.ListDueShiftReceiptUploads(ctx, shiftReceiptUploadBatch)
	if err != nil {
		log.Printf("shift receipt uploads: list due: %v", err)
		return
	}

	for _, row := range rows {
		shiftRef := fmt.Sprintf("shift-%d", row.ShiftID)

		// kashi only has a branch_shifts row to hang the photo on once the
		// shift's own close has synced. Until then, wait -- no backoff bump,
		// this isn't a failure.
		synced, err := a.store.ShiftCloseSyncedForRef(ctx, shiftRef)
		if err != nil {
			log.Printf("shift receipt uploads: %s: check close synced: %v", shiftRef, err)
			continue
		}
		if synced == 0 {
			continue
		}

		if _, statErr := os.Stat(row.FilePath); statErr != nil {
			// The buffered file is gone (manual cleanup, disk swap). Nothing
			// to send and retrying can't fix it -- drop it out of the queue
			// by marking it synced with a note in the log.
			log.Printf("shift receipt uploads: %s: buffered file missing (%v), abandoning", shiftRef, statErr)
			if err := a.store.MarkShiftReceiptSynced(ctx, row.ID); err != nil {
				log.Printf("shift receipt uploads: %s: mark abandoned: %v", shiftRef, err)
			}
			continue
		}

		err = client.PostShiftReceiptImage(ctx, shiftRef, row.FilePath, row.ContentType)
		if err != nil {
			if errors.Is(err, errShiftNotReceivedYet) {
				// Transient by nature: the close is synced locally but kashi
				// is still catching up. Short-circuit to a quick retry.
				a.markShiftReceiptRetry(ctx, row, err)
				continue
			}
			a.markShiftReceiptRetry(ctx, row, err)
			continue
		}

		if err := a.store.MarkShiftReceiptSynced(ctx, row.ID); err != nil {
			log.Printf("shift receipt uploads: %s: mark synced: %v", shiftRef, err)
			continue
		}
		select {
		case <-a.ready:
			runtime.EventsEmit(a.ctx, "shift_receipt_synced:"+row.Token)
		default:
		}
		log.Printf("shift receipt uploads: %s: forwarded to kashi", shiftRef)
	}
}

// markShiftReceiptRetry schedules the next attempt with the same
// exponential backoff (5s doubling, capped at 1h) sync_outbox uses. It never
// moves the row to a terminal state -- the whole point of this table is that
// it keeps trying.
func (a *App) markShiftReceiptRetry(ctx context.Context, row db.ShiftReceiptUpload, cause error) {
	delay := syncBackoffBase << row.AttemptCount
	if delay > syncBackoffMax || delay <= 0 {
		delay = syncBackoffMax
	}
	nextAttemptAt := time.Now().UTC().Add(delay).Format("2006-01-02 15:04:05")
	log.Printf("shift receipt uploads: shift-%d: %v (retrying at %s)", row.ShiftID, cause, nextAttemptAt)
	if err := a.store.MarkShiftReceiptUploadAttemptFailed(ctx, db.MarkShiftReceiptUploadAttemptFailedParams{
		NextAttemptAt: nextAttemptAt,
		ID:            row.ID,
	}); err != nil {
		log.Printf("shift receipt uploads: shift-%d: mark retry: %v", row.ShiftID, err)
	}
}
