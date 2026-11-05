package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

// syncReporter tells kashi how this till's pulls went, for its Commands page:
// which entity failed to apply and why, how many rows landed, and what is still
// waiting to go up. Only the till knows these: kashi can see that a pull repeats,
// not the reason. Purely informational, so it never fails a sync, and a kashi
// too old to have the endpoint just answers 404.
type syncReporter struct {
	mu sync.Mutex
	// The error last reported per entity, to say when one has cleared.
	errors        map[string]string
	outboxPending int64
	outboxFailed  int64
	reportedOnce  bool
}

const (
	syncReportTimeout  = 10 * time.Second
	syncReportMaxError = 500
)

type syncReportEntity struct {
	Entity      string `json:"entity"`
	Error       string `json:"error"`
	RowsApplied int64  `json:"rows_applied"`
}

type syncReportBody struct {
	Entities      []syncReportEntity `json:"entities"`
	OutboxPending int64              `json:"outbox_pending"`
	OutboxFailed  int64              `json:"outbox_failed"`
}

type syncPullResult struct {
	entity  string
	applied int64
	err     error
}

// syncReportEntities picks what a pass has to say: every entity that failed or
// applied something, and any whose earlier error has now gone. A pass that
// applied nothing and changed nothing says nothing.
func (r *syncReporter) syncReportEntities(results []syncPullResult) []syncReportEntity {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errors == nil {
		r.errors = map[string]string{}
	}
	out := []syncReportEntity{}
	for _, res := range results {
		text := ""
		if res.err != nil {
			text = res.err.Error()
			if len(text) > syncReportMaxError {
				text = text[:syncReportMaxError]
			}
		}
		was := r.errors[res.entity]
		if text == "" && was == "" && res.applied == 0 {
			continue
		}
		if text == was && text != "" {
			continue // kashi already has this error
		}
		if text == "" {
			delete(r.errors, res.entity)
		} else {
			r.errors[res.entity] = text
		}
		out = append(out, syncReportEntity{Entity: res.entity, Error: text, RowsApplied: res.applied})
	}
	return out
}

// reportSyncPass sends what a pull pass has to say, if anything.
func (a *App) reportSyncPass(ctx context.Context, client *centralClient, results []syncPullResult) {
	entities := a.syncReport.syncReportEntities(results)
	pending, err := a.store.CountSyncOutboxByStatus(ctx, "pending")
	if err != nil {
		return
	}
	failed, err := a.store.CountSyncOutboxByStatus(ctx, "failed")
	if err != nil {
		return
	}
	r := &a.syncReport
	r.mu.Lock()
	same := r.reportedOnce && r.outboxPending == pending && r.outboxFailed == failed
	r.mu.Unlock()
	if same && len(entities) == 0 {
		return
	}
	if a.sendSyncReport(ctx, client, syncReportBody{Entities: entities, OutboxPending: pending, OutboxFailed: failed}) {
		r.mu.Lock()
		r.reportedOnce, r.outboxPending, r.outboxFailed = true, pending, failed
		r.mu.Unlock()
	}
}

// reportSyncProgress is the running count during a long paged pull.
func (a *App) reportSyncProgress(ctx context.Context, client *centralClient, entity string, applied int64) {
	a.syncReport.mu.Lock()
	pending, failed := a.syncReport.outboxPending, a.syncReport.outboxFailed
	a.syncReport.mu.Unlock()
	a.sendSyncReport(ctx, client, syncReportBody{
		Entities:      []syncReportEntity{{Entity: entity, RowsApplied: applied}},
		OutboxPending: pending, OutboxFailed: failed,
	})
}

func (a *App) sendSyncReport(ctx context.Context, client *centralClient, body syncReportBody) bool {
	ctx, cancel := context.WithTimeout(ctx, syncReportTimeout)
	defer cancel()
	var out map[string]any
	if err := client.doJSON(ctx, http.MethodPost, "/branch/sync/report", body, &out); err != nil {
		log.Printf("sync report: %v", err)
		return false
	}
	return true
}
