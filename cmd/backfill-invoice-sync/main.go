// Command backfill-invoice-sync is a one-time tool: it queues every existing
// invoice (sales and return/exchange) that predates central sync into
// sync_outbox, so the running app's normal background worker picks them up
// and pushes them to kashi on its next tick, same as any new invoice.
//
// It's a separate command, not an App/Wails method, because it's meant to be
// run once per install (or safely re-run -- see below) from a terminal, not
// exposed as a permanent UI action.
//
// The payload shape here must stay identical to branchInvoicePayload in
// sync_outbox.go (the canonical definition, used by the live checkout
// paths) -- it's duplicated rather than imported because that type lives in
// package main at the repo root, which a separate command can't import.
//
// Idempotent: an invoice already carrying a client_ref (in
// invoice_client_refs) was already queued, either by the live checkout path
// or an earlier run of this tool, so it's skipped. Safe to re-run.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"
)

type branchInvoicePayload struct {
	ClientRef              string                     `json:"client_ref"`
	BranchInvoiceCode      string                     `json:"branch_invoice_code"`
	BranchCashboxAccountID int64                      `json:"branch_cashbox_account_id"`
	BranchShiftID          int64                      `json:"branch_shift_id"`
	BranchInventoryID      int64                      `json:"branch_inventory_id"`
	BranchClientID         int64                      `json:"branch_client_id"`
	Discount               int16                      `json:"discount"`
	GrandTotal             int64                      `json:"grand_total"`
	Subtotal               int64                      `json:"sub_total"`
	DiscountedTotal        int64                      `json:"discounted_total"`
	LoyaltyPointsDelta     int64                      `json:"loyalty_points_delta"`
	OccurredAt             time.Time                  `json:"occurred_at"`
	Items                  []branchInvoicePayloadItem `json:"items"`
	RelatedClientRef       string                     `json:"related_client_ref,omitempty"`
}

type branchInvoicePayloadItem struct {
	BranchProductID int64 `json:"branch_product_id"`
	UnitPrice       int64 `json:"unit_price"`
	LineTotal       int64 `json:"line_total"`
	Discount        int16 `json:"discount"`
	Quantity        int64 `json:"quantity"`
}

func main() {
	dbPath := flag.String("db", "", "path to the kashi-pos sqlite database file (required)")
	apply := flag.Bool("apply", false, "actually queue invoices for sync; without this, only reports what would be queued")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("-db is required, e.g. -db ./dbdd/database.db")
	}

	conn, err := sql.Open("sqlite", *dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	store := db.NewStore(conn)
	ctx := context.Background()

	invoices, err := store.ListInvoices(ctx)
	if err != nil {
		log.Fatalf("list invoices: %v", err)
	}
	returns, err := store.ListReturnInvoices(ctx)
	if err != nil {
		log.Fatalf("list return invoices: %v", err)
	}
	salesInvoiceIDByReturnID := make(map[int64]int64, len(returns))
	for _, r := range returns {
		salesInvoiceIDByReturnID[r.InvoiceID] = r.SalesInvoiceID
	}

	queued, skipped := 0, 0
	for _, inv := range invoices {
		did, err := processInvoice(ctx, store, inv, salesInvoiceIDByReturnID, *apply)
		if err != nil {
			log.Fatalf("invoice %d (%s): %v", inv.ID, inv.InvoiceCode, err)
		}
		if did {
			queued++
		} else {
			skipped++
		}
	}

	verb := "would queue"
	if *apply {
		verb = "queued"
	}
	fmt.Printf("%s %d invoice(s) for sync, skipped %d already-synced invoice(s)\n", verb, queued, skipped)
	if !*apply {
		fmt.Println("dry run -- re-run with -apply to actually write")
	}
}

// processInvoice returns true if this invoice was (or would be) newly
// queued, false if it already had a client_ref and was skipped.
func processInvoice(ctx context.Context, store *db.Store, inv db.Invoice, salesInvoiceIDByReturnID map[int64]int64, apply bool) (bool, error) {
	if _, err := store.GetInvoiceClientRef(ctx, inv.ID); err == nil {
		return false, nil // already queued by a live checkout or an earlier run
	} else if err != sql.ErrNoRows {
		return false, fmt.Errorf("check existing client ref: %w", err)
	}

	if !apply {
		return true, nil // dry run: report it, write nothing
	}

	return true, store.ExecTx(ctx, func(q *db.Queries) error {
		clientRef := uuid.NewString()
		if err := q.CreateInvoiceClientRef(ctx, db.CreateInvoiceClientRefParams{InvoiceID: inv.ID, ClientRef: clientRef}); err != nil {
			return fmt.Errorf("create client ref: %w", err)
		}

		products, err := q.ListInvoiceProducts(ctx, inv.ID)
		if err != nil {
			return fmt.Errorf("list invoice products: %w", err)
		}
		items := make([]branchInvoicePayloadItem, 0, len(products))
		for _, p := range products {
			items = append(items, branchInvoicePayloadItem{
				BranchProductID: p.ProductID,
				UnitPrice:       p.UnitPrice,
				LineTotal:       p.LineTotal,
				Discount:        int16(p.Discount),
				Quantity:        p.Quantity,
			})
		}

		occurredAt, err := time.Parse("2006-01-02 15:04:05", inv.CreatedAt)
		if err != nil {
			return fmt.Errorf("parse created_at %q: %w", inv.CreatedAt, err)
		}

		payload := branchInvoicePayload{
			ClientRef:              clientRef,
			BranchInvoiceCode:      inv.InvoiceCode,
			BranchCashboxAccountID: inv.CashboxID,
			BranchShiftID:          inv.ShiftID,
			BranchInventoryID:      inv.InventoryID,
			BranchClientID:         inv.ClientID,
			GrandTotal:             inv.GrandTotal,
			Subtotal:               inv.Subtotal,
			DiscountedTotal:        inv.DiscountedTotal,
			OccurredAt:             occurredAt.UTC(),
			Items:                  items,
		}

		entityType := "sales_invoice"
		if salesInvoiceID, isReturn := salesInvoiceIDByReturnID[inv.ID]; isReturn {
			entityType = "return_invoice"
			relatedRef, err := getOrCreateClientRef(ctx, q, salesInvoiceID)
			if err != nil {
				return fmt.Errorf("resolve source sale's client ref: %w", err)
			}
			payload.RelatedClientRef = relatedRef
			payload.LoyaltyPointsDelta = 0 // matches live checkoutReturnExchangeInvoice: returns never touch loyalty points
		} else {
			payload.LoyaltyPointsDelta = inv.GrandTotal / 100 // matches live salesInvoiceTx's formula exactly
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		if _, err := q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
			EntityType:  entityType,
			ClientRef:   clientRef,
			PayloadJson: string(body),
		}); err != nil {
			return fmt.Errorf("create sync outbox entry: %w", err)
		}
		return nil
	})
}

// getOrCreateClientRef mirrors getOrCreateInvoiceClientRef in sync_outbox.go
// (package main at the repo root, not importable from here) -- needed when
// a return's source sale hasn't been processed by this backfill yet (e.g.
// original sale predates the id range, or was itself skipped for some
// reason) so the return can still carry a valid related_client_ref.
func getOrCreateClientRef(ctx context.Context, q *db.Queries, invoiceID int64) (string, error) {
	ref, err := q.GetInvoiceClientRef(ctx, invoiceID)
	if err == nil {
		return ref, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	ref = uuid.NewString()
	if err := q.CreateInvoiceClientRef(ctx, db.CreateInvoiceClientRefParams{InvoiceID: invoiceID, ClientRef: ref}); err != nil {
		return "", err
	}
	return ref, nil
}
