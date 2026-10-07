package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// closeShiftRequiresSyncFlag: when on (and sync is configured), a shift can't
// be closed -- and its receipt isn't printed -- while anything is still
// waiting in sync_outbox. The close-shift screen runs FlushSyncOutbox first
// and shows its progress. Default (missing row) is off. kashi can set it per
// branch through update_settings.
const closeShiftRequiresSyncFlag = "close_shift_requires_sync"

const (
	// syncFlushConcurrency is how many rows a flush has in flight at once.
	// The live drain sends one at a time so a fresh sale is never stuck
	// behind a backlog; a flush has a cashier waiting on the whole backlog,
	// so it trades that for throughput -- a day offline is hundreds of
	// rows, and serially each one is a full round trip.
	syncFlushConcurrency = 8
	syncFlushBatchSize   = 500
	// syncFlushMaxRounds bounds the retries of rows kashi answered with a
	// 5xx/429 mid-flush (they go on per-row backoff). Past that the flush
	// gives up and reports them; the live push loop keeps trying.
	syncFlushMaxRounds    = 4
	syncFlushProgressTick = 300 * time.Millisecond
)

func (a *App) closeShiftRequiresSync() bool {
	return a.flagEnabled(a.ctx, closeShiftRequiresSyncFlag)
}

// SetCloseShiftRequiresSyncEnabled backs the Settings toggle (see
// closeShiftRequiresSyncFlag).
func (a *App) SetCloseShiftRequiresSyncEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, closeShiftRequiresSyncFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// SyncPendingByType is one row of the "still to upload" breakdown.
type SyncPendingByType struct {
	EntityType string `json:"entityType"`
	Count      int64  `json:"count"`
}

// CloseShiftSyncGate is what the close-shift screen checks before closing.
// Required is false when the mode is off or this till doesn't sync at all;
// the screen then closes straight away as before.
type CloseShiftSyncGate struct {
	Required bool                `json:"required"`
	Pending  int64               `json:"pending"`
	Failed   int64               `json:"failed"`
	ByType   []SyncPendingByType `json:"byType"`
}

func (a *App) GetCloseShiftSyncGate() (CloseShiftSyncGate, error) {
	if a.store == nil {
		return CloseShiftSyncGate{}, fmt.Errorf("database not initialized")
	}
	required, err := a.closeShiftSyncRequired()
	if err != nil || !required {
		return CloseShiftSyncGate{}, err
	}
	gate := CloseShiftSyncGate{Required: true}
	if gate.ByType, gate.Pending, err = a.pendingSyncByType(a.ctx); err != nil {
		return gate, err
	}
	if gate.Failed, err = a.store.CountSyncOutboxByStatus(a.ctx, "failed"); err != nil {
		return gate, err
	}
	return gate, nil
}

func (a *App) closeShiftSyncRequired() (bool, error) {
	if !a.closeShiftRequiresSync() {
		return false, nil
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return false, err
	}
	return centralURL != "", nil
}

func (a *App) pendingSyncByType(ctx context.Context) ([]SyncPendingByType, int64, error) {
	rows, err := a.store.CountPendingSyncOutboxByEntityType(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]SyncPendingByType, 0, len(rows))
	var total int64
	for _, r := range rows {
		out = append(out, SyncPendingByType{EntityType: r.EntityType, Count: r.Count})
		total += r.Count
	}
	return out, total, nil
}

// errCloseShiftSyncPending is the message prefix CloseShiftByID refuses
// with; the close-shift screen matches on it.
const errCloseShiftSyncPending = "sync pending"

// checkCloseShiftSyncGate is CloseShiftByID's server-side half of the gate:
// the screen flushes first, but a close must not slip through if it didn't
// (an older frontend, or rows queued between the flush and the click). A
// manager bypass for this exact shift lets one close through.
func (a *App) checkCloseShiftSyncGate(shiftID int64) error {
	required, err := a.closeShiftSyncRequired()
	if err != nil || !required {
		return err
	}
	if a.closeShiftSyncBypass.CompareAndSwap(shiftID, 0) {
		return nil
	}
	pending, err := a.store.CountSyncOutboxByStatus(a.ctx, "pending")
	if err != nil {
		return err
	}
	if pending > 0 {
		return fmt.Errorf("%s: %d item(s) still waiting to upload to kashi", errCloseShiftSyncPending, pending)
	}
	return nil
}

// BypassCloseShiftSyncGate lets the next close of shiftID through without
// everything uploaded -- the escape hatch for a branch that's genuinely
// offline. Needs the admin password; nothing is lost, the rows stay queued
// and upload whenever kashi is reachable again.
func (a *App) BypassCloseShiftSyncGate(shiftID int64, adminPassword, reason string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if hashPassword(adminPassword) != a.adminPasswordHash() {
		return fmt.Errorf("wrong admin password")
	}
	pending, _ := a.store.CountSyncOutboxByStatus(a.ctx, "pending")
	log.Printf("close shift %d: sync gate bypassed with %d item(s) pending (reason: %q)",
		shiftID, pending, strings.TrimSpace(reason))
	a.closeShiftSyncBypass.Store(shiftID)
	return nil
}

// SyncFlushProgress is emitted as "sync_flush_progress" while a flush runs.
// Counts come from the outbox itself, so rows queued mid-flush (a sale on
// another screen, an attendance punch) simply grow Total.
type SyncFlushProgress struct {
	Total      int64               `json:"total"`
	Done       int64               `json:"done"`
	Failed     int64               `json:"failed"`
	Remaining  int64               `json:"remaining"`
	PerSecond  float64             `json:"perSecond"`
	EtaSeconds float64             `json:"etaSeconds"` // -1 while there's no rate yet
	Elapsed    float64             `json:"elapsedSeconds"`
	ByType     []SyncPendingByType `json:"byType"`
	Finished   bool                `json:"finished"`
}

// FlushSyncOutbox uploads everything waiting in sync_outbox right now, as
// fast as kashi will take it, and returns once the outbox is empty (or it
// can't get further). It's the "Sync now" behind the close-shift gate:
//
//   - ignores every backoff: the connection's (pushLink) and each row's;
//   - sends syncFlushConcurrency rows at a time, in dependency waves so a
//     return or settlement never reaches kashi before its sale;
//   - keeps the live push loop out meanwhile, so no row is sent twice.
//
// Rows kashi rejects outright still go to 'failed' -- retrying identical
// bytes won't change the answer -- and are reported, not waited on.
func (a *App) FlushSyncOutbox() (SyncFlushProgress, error) {
	if a.store == nil {
		return SyncFlushProgress{}, fmt.Errorf("database not initialized")
	}
	client := a.syncClient(a.ctx, "flush")
	if client == nil {
		return SyncFlushProgress{}, fmt.Errorf("sync with kashi isn't configured")
	}
	transport := flushTransport()
	defer transport.CloseIdleConnections()
	client.httpClient.Transport = transport

	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	a.syncFlushMu.Lock()
	if a.syncFlushCancel != nil {
		a.syncFlushMu.Unlock()
		return SyncFlushProgress{}, fmt.Errorf("an upload is already running")
	}
	a.syncFlushCancel = cancel
	a.syncFlushMu.Unlock()
	defer func() {
		a.syncFlushMu.Lock()
		a.syncFlushCancel = nil
		a.syncFlushMu.Unlock()
	}()

	// Keep the live push loop out (it TryLocks), and wait for a pass that's
	// already mid-row to finish.
	a.pushMu.Lock()
	defer a.pushMu.Unlock()

	if err := client.Health(ctx); err != nil && isNetworkError(err) {
		a.pushLink.markDown(time.Now())
		return SyncFlushProgress{}, fmt.Errorf("can't reach kashi -- check the internet connection")
	}
	a.pushLink.markUp()
	if err := a.releaseBackedOffOutbox(ctx); err != nil {
		return SyncFlushProgress{}, err
	}

	tracker, err := a.newFlushTracker(ctx)
	if err != nil {
		return SyncFlushProgress{}, err
	}
	stopTicker := tracker.emitEvery(syncFlushProgressTick)
	defer a.emitSyncStateChanged()

	runErr := a.flushOutbox(ctx, client)
	stopTicker()

	final := tracker.snapshot()
	final.Finished = true
	a.emitSyncFlushProgress(final)
	if runErr != nil {
		return final, runErr
	}
	if final.Remaining > 0 {
		return final, fmt.Errorf("%d item(s) couldn't be uploaded yet -- kashi kept answering with an error", final.Remaining)
	}
	return final, nil
}

// CancelSyncFlush stops a running FlushSyncOutbox after the rows already in
// flight. Nothing is lost -- whatever's left stays queued for the live loop.
func (a *App) CancelSyncFlush() {
	a.syncFlushMu.Lock()
	defer a.syncFlushMu.Unlock()
	if a.syncFlushCancel != nil {
		a.syncFlushCancel()
	}
}

func (a *App) releaseBackedOffOutbox(ctx context.Context) error {
	return retryBusy(func() error {
		_, err := a.store.MakePendingSyncOutboxDueNow(ctx)
		return err
	})
}

func (a *App) flushOutbox(ctx context.Context, client *centralClient) error {
	downsAtStart := a.pushLink.downCount()
	linkFailed := func() bool { return a.pushLink.downCount() != downsAtStart }

	for round := 0; round < syncFlushMaxRounds; round++ {
		if round > 0 {
			// Rows kashi answered with a 5xx/429 are on per-row backoff; give
			// it a moment, then make them due again.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(round) * 2 * time.Second):
			}
			if err := a.releaseBackedOffOutbox(ctx); err != nil {
				return err
			}
		}
		for {
			entries, err := a.store.ListDueSyncOutboxEntries(ctx, syncFlushBatchSize)
			if err != nil {
				return fmt.Errorf("list due outbox entries: %w", err)
			}
			if len(entries) == 0 {
				break
			}
			a.flushBatch(ctx, client, entries, linkFailed)
			if linkFailed() {
				return fmt.Errorf("lost the connection to kashi -- check the internet and try again")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		pending, err := a.store.CountSyncOutboxByStatus(ctx, "pending")
		if err != nil {
			return err
		}
		if pending == 0 {
			return nil
		}
	}
	return nil
}

// flushWave orders a batch into dependency waves: sales first, then what
// points at a sale (returns, settlements), then the other transactional
// rows, then everything else. Rows inside a wave don't depend on each other
// and go out in parallel. ListDueSyncOutboxEntries' tier order already puts
// the same things first, so a batch boundary never splits a dependency the
// wrong way round.
func flushWave(entityType string) int {
	switch entityType {
	case "sales_invoice":
		return 0
	case "return_invoice", "invoice_settlement":
		return 1
	case "shift_close", "expense", "loan", "visitor_event", "attendance_events", "attendance_change":
		return 2
	default:
		return 3
	}
}

func (a *App) flushBatch(ctx context.Context, client *centralClient, entries []db.SyncOutbox, linkFailed func() bool) {
	var waves [4][]db.SyncOutbox
	for _, e := range entries {
		w := flushWave(e.EntityType)
		waves[w] = append(waves[w], e)
	}
	for _, wave := range waves {
		if linkFailed() || ctx.Err() != nil {
			return
		}
		// The two batched routes stay batched: one POST for the lot beats
		// any amount of parallelism.
		var visitors, clients, rest []db.SyncOutbox
		for _, e := range wave {
			switch e.EntityType {
			case "visitor_event":
				visitors = append(visitors, e)
			case "client":
				clients = append(clients, e)
			default:
				rest = append(rest, e)
			}
		}
		if len(clients) < 2 {
			rest = append(rest, clients...)
			clients = nil
		}

		var wg sync.WaitGroup
		if len(visitors) > 0 {
			wg.Add(1)
			go func() { defer wg.Done(); a.drainVisitorEventBatch(ctx, client, visitors) }()
		}
		if len(clients) > 0 {
			wg.Add(1)
			go func() { defer wg.Done(); a.drainClientBatch(ctx, client, clients) }()
		}
		sem := make(chan struct{}, syncFlushConcurrency)
		for _, e := range rest {
			if linkFailed() || ctx.Err() != nil {
				break
			}
			sem <- struct{}{}
			wg.Add(1)
			go func(e db.SyncOutbox) {
				defer wg.Done()
				defer func() { <-sem }()
				a.pushOutboxEntry(ctx, client, e)
			}(e)
		}
		wg.Wait()
	}
}

// flushTransport is the default transport with enough idle connections kept
// per host for syncFlushConcurrency requests -- the default keeps 2, so the
// rest would each pay a fresh TCP+TLS handshake per row.
func flushTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = syncFlushConcurrency * 2
	t.MaxConnsPerHost = syncFlushConcurrency * 2
	return t
}

// flushTracker turns outbox counts into progress: done is how many rows left
// 'pending' (synced or failed) since the flush began.
type flushTracker struct {
	a            *App
	ctx          context.Context
	start        time.Time
	syncedAtZero int64
	failedAtZero int64
	peakTotal    int64
}

func (a *App) newFlushTracker(ctx context.Context) (*flushTracker, error) {
	t := &flushTracker{a: a, ctx: ctx, start: time.Now()}
	var err error
	if t.syncedAtZero, err = a.store.CountSyncOutboxByStatus(ctx, "synced"); err != nil {
		return nil, err
	}
	if t.failedAtZero, err = a.store.CountSyncOutboxByStatus(ctx, "failed"); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *flushTracker) snapshot() SyncFlushProgress {
	// The flush's own ctx may be cancelled by now; counting must still work.
	ctx := t.a.ctx
	var p SyncFlushProgress
	synced, err1 := t.a.store.CountSyncOutboxByStatus(ctx, "synced")
	failed, err2 := t.a.store.CountSyncOutboxByStatus(ctx, "failed")
	byType, remaining, err3 := t.a.pendingSyncByType(ctx)
	if err := errors.Join(err1, err2, err3); err != nil {
		log.Printf("sync flush progress: %v", err)
	}
	p.Failed = max(failed-t.failedAtZero, 0)
	p.Done = max(synced-t.syncedAtZero, 0) + p.Failed
	p.Remaining = remaining
	p.ByType = byType
	t.peakTotal = max(t.peakTotal, p.Done+p.Remaining)
	p.Total = t.peakTotal
	p.Elapsed = time.Since(t.start).Seconds()
	p.EtaSeconds = -1
	if p.Done > 0 && p.Elapsed > 0.5 {
		p.PerSecond = float64(p.Done) / p.Elapsed
		p.EtaSeconds = float64(p.Remaining) / p.PerSecond
	}
	return p
}

// emitEvery emits a snapshot right away and then every interval until the
// returned stop is called.
func (t *flushTracker) emitEvery(interval time.Duration) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			t.a.emitSyncFlushProgress(t.snapshot())
			select {
			case <-done:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

func (a *App) emitSyncFlushProgress(p SyncFlushProgress) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "sync_flush_progress", p)
	default:
	}
}
