package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	db "kashi-pos/db/sqlc"
)

// syncReporter tells central how this till's pulls went, for its Commands page:
// which entity failed to apply and why, how many rows landed, and what is still
// waiting to go up. Only the till knows these: central can see that a pull repeats,
// not the reason. Purely informational, so it never fails a sync, and a central
// too old to have the endpoint just answers 404.
type syncReporter struct {
	mu sync.Mutex
	// The error last reported per entity, to say when one has cleared.
	errors        map[string]string
	outboxPending int64
	outboxFailed  int64
	reportedOnce  bool
	// The lists last reported, to send them only when one has changed.
	listsSent string
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

// syncReportList is one list as this till holds it. Central works out the same from
// what it has, so comparing the two proves the till holds the right rows, where its
// sync cursor only says what it should have received.
type syncReportList struct {
	Kind string `json:"kind"`
	UID  string `json:"uid"`
	Rows int    `json:"rows"`
	Hash string `json:"hash"`
}

type syncReportBody struct {
	Entities      []syncReportEntity `json:"entities"`
	Lists         []syncReportList   `json:"lists,omitempty"`
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
			continue // central already has this error
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
	lists, err := a.syncReportLists(ctx, a.store.Queries)
	if err != nil {
		log.Printf("sync report lists: %v", err)
		lists = nil
	}
	listsKey := ""
	if key, err := json.Marshal(lists); err == nil {
		listsKey = string(key)
	}
	r := &a.syncReport
	r.mu.Lock()
	same := r.reportedOnce && r.outboxPending == pending && r.outboxFailed == failed
	if r.listsSent == listsKey {
		lists = nil // central already has these
	}
	r.mu.Unlock()
	if same && len(entities) == 0 && len(lists) == 0 {
		return
	}
	if a.sendSyncReport(ctx, client, syncReportBody{Entities: entities, Lists: lists, OutboxPending: pending, OutboxFailed: failed}) {
		r.mu.Lock()
		r.reportedOnce, r.outboxPending, r.outboxFailed = true, pending, failed
		if len(lists) > 0 {
			r.listsSent = listsKey
		}
		r.mu.Unlock()
	}
}

// syncReportLists fingerprints every list in force on this till.
func (a *App) syncReportLists(ctx context.Context, q *db.Queries) ([]syncReportList, error) {
	out := []syncReportList{}
	priceLists, err := q.ListPriceLists(ctx)
	if err != nil {
		return nil, err
	}
	for _, list := range priceLists {
		if list.IsActive == 0 || !list.Uid.Valid {
			continue
		}
		rows, err := q.ListPriceListItems(ctx, list.ID)
		if err != nil {
			return nil, err
		}
		count, hash := priceListFingerprint(rows)
		out = append(out, syncReportList{Kind: "price", UID: list.Uid.String, Rows: count, Hash: hash})
	}
	discountLists, err := q.ListDiscountLists(ctx)
	if err != nil {
		return nil, err
	}
	for _, list := range discountLists {
		if list.IsActive == 0 || !list.Uid.Valid {
			continue
		}
		rows, err := q.ListDiscountListItems(ctx, list.ID)
		if err != nil {
			return nil, err
		}
		options, err := q.ListDiscountListOptionItems(ctx, list.ID)
		if err != nil {
			return nil, err
		}
		count, hash := discountListFingerprint(rows, options)
		out = append(out, syncReportList{Kind: "discount", UID: list.Uid.String, Rows: count, Hash: hash})
	}
	return out, nil
}

// A fingerprint is a list's row count and a hash of its rows, worked out the same
// way here and in central (api/list_fingerprint.go; a test in each repo checks the same
// rows give the same hash). One line per row, sorted, so row order can't change it:
//
//	price list:     "<variant uid>|<price>"
//	discount list:  "<variant uid>|<discount>|<deal uid>|<deal fallback>" (a row
//	                with no deal or fallback leaves those empty), then its option
//	                rows as "o|<variant uid>|<sale option>|<discount>"
func fingerprintLines(lines []string) (int, string) {
	sort.Strings(lines)
	h := sha256.New()
	for _, line := range lines {
		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}
	return len(lines), hex.EncodeToString(h.Sum(nil))
}

func priceListFingerprint(rows []db.PriceListItem) (int, string) {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s|%d", r.ProductUid, r.Price))
	}
	return fingerprintLines(lines)
}

func discountListFingerprint(rows []db.DiscountListItem, options []db.DiscountListOptionItem) (int, string) {
	lines := make([]string, 0, len(rows)+len(options))
	for _, r := range rows {
		fallback := ""
		if r.DealUid != "" && r.DealFallbackPercent.Valid {
			fallback = strconv.FormatInt(r.DealFallbackPercent.Int64, 10)
		}
		lines = append(lines, fmt.Sprintf("%s|%d|%s|%s", r.ProductUid, r.Discount, r.DealUid, fallback))
	}
	for _, o := range options {
		lines = append(lines, fmt.Sprintf("o|%s|%s|%d", o.ProductUid, o.SaleOption, o.Discount))
	}
	return fingerprintLines(lines)
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
