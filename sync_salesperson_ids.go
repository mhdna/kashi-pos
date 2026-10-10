package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// kashi used to learn only a sale's salesperson *name*, so renaming a
// salesperson split them into two people on kashi's Salesperson Performance
// page. Every invoice now also carries kashi's id for the salesperson (the
// local salespersons.remote_id), and once per till the ids of all past
// sales are reported in bulk.

// salespersonIDsBatch is how many past invoices go in one report request.
const salespersonIDsBatch = 1000

// salespersonIDsRetry spaces out report attempts that didn't go through
// (an older kashi without the endpoint, or kashi unreachable).
const salespersonIDsRetry = time.Hour

// salespersonRemoteID is kashi's id for a local salesperson; 0 when there is
// none or the salesperson isn't one of kashi's.
func salespersonRemoteID(ctx context.Context, q *db.Queries, localID int64) int64 {
	if localID <= 0 {
		return 0
	}
	sp, err := q.GetSalespersonByID(ctx, localID)
	if err != nil || !sp.RemoteID.Valid {
		return 0
	}
	return sp.RemoteID.Int64
}

// salespersonRemoteIDByName is the same for the salesperson picked by name
// at checkout (the app bar's selection).
func salespersonRemoteIDByName(ctx context.Context, q *db.Queries, name string) int64 {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}
	people, err := q.ListSalespersons(ctx)
	if err != nil {
		return 0
	}
	for _, sp := range people {
		if sp.FullName == name && sp.RemoteID.Valid {
			return sp.RemoteID.Int64
		}
	}
	return 0
}

type invoiceSalespersonReport struct {
	BranchInvoiceCode string `json:"branch_invoice_code"`
	SalespersonID     int64  `json:"salesperson_id"`
}

// reportInvoiceSalespersons sends kashi the salesperson of every past sale,
// once. It waits for the outbox to drain: a sale still queued there isn't on
// kashi yet, and one queued before this version has no id in its payload.
func (a *App) reportInvoiceSalespersons(ctx context.Context, client *centralClient) {
	now := time.Now()
	if next := a.salespersonIDsNextTry.Load(); next != 0 && now.Unix() < next {
		return
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil || settings.SalespersonIdsReportedAt.Valid {
		return
	}
	if pending, err := a.store.CountPendingSyncOutbox(ctx); err != nil || pending > 0 {
		return
	}

	if err := a.sendInvoiceSalespersons(ctx, client); err != nil {
		a.salespersonIDsNextTry.Store(now.Add(salespersonIDsRetry).Unix())
		log.Printf("salesperson ids: report past sales: %v", err)
		return
	}
	stamp := sql.NullString{String: now.UTC().Format(time.RFC3339), Valid: true}
	if err := a.store.SetSalespersonIDsReportedAt(ctx, stamp); err != nil {
		log.Printf("salesperson ids: mark reported: %v", err)
	}
}

func (a *App) sendInvoiceSalespersons(ctx context.Context, client *centralClient) error {
	rows, err := a.store.ListInvoiceSalespersonRemoteIDs(ctx)
	if err != nil {
		return fmt.Errorf("list invoices: %w", err)
	}
	updated := 0
	for start := 0; start < len(rows); start += salespersonIDsBatch {
		end := min(start+salespersonIDsBatch, len(rows))
		items := make([]invoiceSalespersonReport, 0, end-start)
		for _, row := range rows[start:end] {
			items = append(items, invoiceSalespersonReport{
				BranchInvoiceCode: row.InvoiceCode,
				SalespersonID:     row.RemoteID.Int64,
			})
		}
		body, err := json.Marshal(map[string]any{"items": items})
		if err != nil {
			return err
		}
		if err := client.sendJSON(ctx, "POST", "/branch/invoice_salespersons", body); err != nil {
			return err
		}
		updated += len(items)
	}
	log.Printf("salesperson ids: reported %d past sales to kashi", updated)
	return nil
}
