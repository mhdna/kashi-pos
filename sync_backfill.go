package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// ResyncOptions controls one "sync everything now" run.
type ResyncOptions struct {
	// ApplyStock pushes backfilled invoices as normal sales that move
	// central stock, instead of as historical records that don't. Only
	// correct when kashi's inventory was seeded before the oldest local
	// sale -- off by default.
	ApplyStock bool `json:"applyStock"`
	// DryRun reports what would be queued without writing anything.
	DryRun bool `json:"dryRun"`
}

// ResyncEntityReport is one row of a ResyncReport: how many of this entity
// exist locally, how many kashi already had, how many were missing, and how
// many this run queued (0 on a dry run). Local vs Central diverging by more
// than Missing after a clean run is the tell that rows were deleted on one
// side -- there is no delete propagation.
type ResyncEntityReport struct {
	Entity  string `json:"entity"`
	Local   int    `json:"local"`
	Central int    `json:"central"`
	Missing int    `json:"missing"`
	Queued  int    `json:"queued"`
}

type ResyncReport struct {
	Target     string               `json:"target"`
	DryRun     bool                 `json:"dryRun"`
	Entities   []ResyncEntityReport `json:"entities"`
	Conflicts  int                  `json:"conflicts"`
	StartedAt  time.Time            `json:"startedAt"`
	FinishedAt time.Time            `json:"finishedAt"`
}

// ResyncAll brings this branch's data to parity with the central server it's
// currently pointed at: it pulls master data down from scratch, then pushes
// up every local client and invoice the server is missing (asking it first,
// via reconcile, rather than blindly re-sending), and re-arms any other
// outbox rows that a *previous* central server had accepted. Safe to re-run
// and safe to run right after changing the central URL -- that's its point.
//
// Products flow up through the conflict-review path (unknown_product), not
// here: a central product carries colours/sizes/categories the flat POS row
// can't fully supply, so an operator resolves each one in kashi.
func (a *App) ResyncAll(opts ResyncOptions) (ResyncReport, error) {
	if a.store == nil {
		return ResyncReport{}, fmt.Errorf("database not initialized")
	}
	if !a.resyncMu.TryLock() {
		return ResyncReport{}, fmt.Errorf("a sync-everything run is already in progress")
	}
	defer a.resyncMu.Unlock()

	ctx := a.ctx
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return ResyncReport{}, fmt.Errorf("load settings: %w", err)
	}
	if settings.CentralApiUrl == "" {
		return ResyncReport{}, fmt.Errorf("central sync is not configured -- set the server URL and API key first")
	}
	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	if !client.enabled() {
		return ResyncReport{}, fmt.Errorf("central sync is not fully configured (URL, branch code and API key are all required)")
	}

	report := ResyncReport{
		Target:    client.baseURL,
		DryRun:    opts.DryRun,
		StartedAt: time.Now().UTC(),
	}

	// 1. Pull master data down from the beginning. A missing sync_cursors
	//    row already means "everything", so clearing them and running one
	//    pull pass is the whole "brand-new till catches up" story.
	if !opts.DryRun {
		if err := a.store.ClearSyncCursors(ctx); err != nil {
			return report, fmt.Errorf("clear sync cursors: %w", err)
		}
		a.emitResyncProgress("pull", 0, 0)
		a.runSyncPullOnce(ctx, client)
	}

	// 2. Clients up.
	clientsRep, err := a.resyncClients(ctx, client, opts)
	if err != nil {
		return report, fmt.Errorf("resync clients: %w", err)
	}
	report.Entities = append(report.Entities, clientsRep)

	// 3. Invoices up (sales + returns/exchanges).
	invRep, err := a.resyncInvoices(ctx, client, opts)
	if err != nil {
		return report, fmt.Errorf("resync invoices: %w", err)
	}
	report.Entities = append(report.Entities, invRep)

	// 4. Products up -- staged for review in kashi (filled by Phase 4).
	prodRep, err := a.resyncProducts(ctx, client, opts)
	if err != nil {
		return report, fmt.Errorf("resync products: %w", err)
	}
	report.Entities = append(report.Entities, prodRep)

	// 4b. Expenses and loans up -- both carry an on-row client_ref, so
	//     reconstructing a pre-sync one is a straight column map.
	expRep, err := a.resyncExpenses(ctx, client, opts)
	if err != nil {
		return report, fmt.Errorf("resync expenses: %w", err)
	}
	report.Entities = append(report.Entities, expRep)

	loanRep, err := a.resyncLoans(ctx, client, opts)
	if err != nil {
		return report, fmt.Errorf("resync loans: %w", err)
	}
	report.Entities = append(report.Entities, loanRep)

	// 5. Everything else already in the outbox: re-arm rows a previous
	//    server confirmed (or that hard-failed) so they re-push to this one.
	rearmed := 0
	if !opts.DryRun {
		n, err := a.store.RequeueSyncOutboxConfirmedElsewhere(ctx, client.baseURL)
		if err != nil {
			return report, fmt.Errorf("re-arm outbox: %w", err)
		}
		rearmed = int(n)
	}
	report.Entities = append(report.Entities, ResyncEntityReport{
		Entity: "other_outbox_rows", Queued: rearmed,
	})

	// 6. Stamp state + kick the worker to start draining.
	if !opts.DryRun {
		if err := a.store.SetSyncTarget(ctx, client.baseURL); err != nil {
			log.Printf("resync: set sync target: %v", err)
		}
		reportJSON, _ := json.Marshal(report)
		if err := a.store.SetLastReconcile(ctx, string(reportJSON)); err != nil {
			log.Printf("resync: record report: %v", err)
		}
		a.kickSyncWorker()
		a.emitSyncStateChanged()
	}

	report.FinishedAt = time.Now().UTC()
	a.emitResyncProgress("done", 0, 0)
	return report, nil
}

func (a *App) resyncClients(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "clients"}

	clients, err := a.store.ListClients(ctx)
	if err != nil {
		return rep, err
	}
	refs := make([]string, 0, len(clients))
	byRef := make(map[string]db.Client, len(clients))
	for _, c := range clients {
		if isWalkInClient(c) {
			continue
		}
		ref := strconv.FormatInt(c.ID, 10)
		refs = append(refs, ref)
		byRef[ref] = c
	}
	rep.Local = len(refs)

	missing, err := a.reconcileEntity(ctx, client, "clients", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("clients", rep.Central, rep.Local)

	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		c := byRef[ref]
		body, err := json.Marshal(branchClientPayload{
			BranchClientID: c.ID,
			Name:           c.Name,
			Phone:          c.Phone,
		})
		if err != nil {
			return rep, err
		}
		// A stable, resync-scoped ref so a re-run upserts the same row
		// rather than colliding on the table-wide UNIQUE(client_ref); it
		// deliberately can't clash with the live path's
		// "client-<id>-<nanos>" throwaway refs.
		if err := a.store.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType:  "client",
			ClientRef:   "client-resync-" + ref,
			PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

func (a *App) resyncInvoices(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "invoices"}

	invoices, err := a.store.ListInvoices(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(invoices)

	returns, err := a.store.ListReturnInvoices(ctx)
	if err != nil {
		return rep, err
	}
	salesIDByReturnID := make(map[int64]int64, len(returns))
	for _, r := range returns {
		salesIDByReturnID[r.InvoiceID] = r.SalesInvoiceID
	}

	// Existing refs (invoices already known to sync, live or a prior resync).
	existing, err := a.store.ListInvoiceClientRefsMap(ctx)
	if err != nil {
		return rep, err
	}
	refByInvoice := make(map[int64]string, len(existing))
	for _, e := range existing {
		refByInvoice[e.InvoiceID] = e.ClientRef
	}

	if opts.DryRun {
		// Anything without a ref was never synced; anything with one, ask.
		var known []string
		for _, inv := range invoices {
			if ref, ok := refByInvoice[inv.ID]; ok {
				known = append(known, ref)
			} else {
				rep.Missing++
			}
		}
		missing, err := a.reconcileEntity(ctx, client, "invoices", known)
		if err != nil {
			return rep, err
		}
		rep.Missing += len(missing)
		rep.Central = rep.Local - rep.Missing
		a.emitResyncProgress("invoices", rep.Central, rep.Local)
		return rep, nil
	}

	// Mint a stable ref for every invoice that lacks one, so the whole set
	// can be reconciled by ref in one shot.
	if err := a.store.ExecTx(ctx, func(q *db.Queries) error {
		for _, inv := range invoices {
			if _, ok := refByInvoice[inv.ID]; ok {
				continue
			}
			ref, err := getOrCreateInvoiceClientRef(ctx, q, inv.ID)
			if err != nil {
				return err
			}
			refByInvoice[inv.ID] = ref
		}
		return nil
	}); err != nil {
		return rep, fmt.Errorf("mint invoice refs: %w", err)
	}

	allRefs := make([]string, 0, len(invoices))
	invByRef := make(map[string]db.Invoice, len(invoices))
	for _, inv := range invoices {
		ref := refByInvoice[inv.ID]
		allRefs = append(allRefs, ref)
		invByRef[ref] = inv
	}

	missing, err := a.reconcileEntity(ctx, client, "invoices", allRefs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("invoices", rep.Central, rep.Local)

	for i, ref := range missing {
		inv := invByRef[ref]
		salesID, isReturn := salesIDByReturnID[inv.ID]
		if err := a.store.ExecTx(ctx, func(q *db.Queries) error {
			payload, err := buildBranchInvoicePayloadFromDB(ctx, q, inv, isReturn, salesID, !opts.ApplyStock)
			if err != nil {
				return err
			}
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			entityType := "sales_invoice"
			if isReturn {
				entityType = "return_invoice"
			}
			return q.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
				EntityType:  entityType,
				ClientRef:   ref,
				PayloadJson: string(body),
			})
		}); err != nil {
			return rep, fmt.Errorf("queue invoice %d: %w", inv.ID, err)
		}
		rep.Queued++
		if i%50 == 0 {
			a.emitResyncProgress("invoices", rep.Central+rep.Queued, rep.Local)
		}
	}
	return rep, nil
}

// buildBranchInvoicePayloadFromDB reconstructs the exact push payload for an
// already-persisted invoice, purely from its stored rows -- the backfill's
// counterpart to what the live checkout path assembles from data it still
// has in memory. It reuses getOrCreateInvoiceClientRef and
// buildBranchInvoicePayments so the frozen JSON matches the live shape
// (barcode on every line, the per-account payment split, salesperson name).
func buildBranchInvoicePayloadFromDB(ctx context.Context, q *db.Queries, inv db.Invoice, isReturn bool, salesInvoiceID int64, historical bool) (branchInvoicePayload, error) {
	ref, err := getOrCreateInvoiceClientRef(ctx, q, inv.ID)
	if err != nil {
		return branchInvoicePayload{}, fmt.Errorf("resolve client ref: %w", err)
	}

	lines, err := q.ListInvoiceProducts(ctx, inv.ID)
	if err != nil {
		return branchInvoicePayload{}, fmt.Errorf("list invoice products: %w", err)
	}
	items := make([]branchInvoicePayloadItem, 0, len(lines))
	for _, line := range lines {
		barcode := ""
		if prod, err := q.GetProductByID(ctx, line.ProductID); err == nil {
			barcode = prod.Barcode
		}
		items = append(items, branchInvoicePayloadItem{
			BranchProductID: line.ProductID,
			Barcode:         barcode,
			UnitPrice:       line.UnitPrice,
			LineTotal:       line.LineTotal,
			Discount:        int16(line.Discount),
			Quantity:        line.Quantity,
		})
	}

	payments, err := buildBranchInvoicePayments(ctx, q, inv.ID)
	if err != nil {
		return branchInvoicePayload{}, err
	}

	salespersonName := ""
	if inv.SalespersonID != 0 {
		if sp, err := q.GetSalespersonByID(ctx, inv.SalespersonID); err == nil {
			salespersonName = sp.FullName
		}
	}

	occurredAt := time.Now().UTC()
	if t, err := time.Parse("2006-01-02 15:04:05", inv.CreatedAt); err == nil {
		occurredAt = t.UTC()
	}

	loyaltyDelta := int64(0)
	if !isReturn {
		loyaltyDelta = inv.GrandTotal / 100
		if c, err := q.GetClientByID(ctx, inv.ClientID); err == nil && isWalkInClient(c) {
			loyaltyDelta = 0
		}
	}

	payload := branchInvoicePayload{
		ClientRef:              ref,
		BranchInvoiceCode:      inv.InvoiceCode,
		BranchCashboxAccountID: inv.CashboxID,
		BranchShiftID:          inv.ShiftID,
		BranchInventoryID:      inv.InventoryID,
		BranchClientID:         inv.ClientID,
		Discount:               int16(inv.Discount),
		GrandTotal:             inv.GrandTotal,
		Subtotal:               inv.Subtotal,
		DiscountedTotal:        inv.DiscountedTotal,
		LoyaltyPointsDelta:     loyaltyDelta,
		OccurredAt:             occurredAt,
		SalespersonName:        salespersonName,
		Payments:               payments,
		Items:                  items,
		Historical:             historical,
	}
	if isReturn {
		relatedRef, err := getOrCreateInvoiceClientRef(ctx, q, salesInvoiceID)
		if err != nil {
			return branchInvoicePayload{}, fmt.Errorf("resolve source sale ref: %w", err)
		}
		payload.RelatedClientRef = relatedRef
	}
	return payload, nil
}

func parseLocalTime(v string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

func (a *App) resyncExpenses(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "expenses"}
	rows, err := a.store.ListAllExpenses(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(rows)
	byRef := make(map[string]db.Expense, len(rows))
	refs := make([]string, 0, len(rows))
	for _, r := range rows {
		byRef[r.ClientRef] = r
		refs = append(refs, r.ClientRef)
	}

	missing, err := a.reconcileEntity(ctx, client, "expenses", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("expenses", rep.Central, rep.Local)
	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		r := byRef[ref]
		var shiftID *int64
		if r.ShiftID.Valid {
			shiftID = &r.ShiftID.Int64
		}
		body, err := json.Marshal(branchExpensePayload{
			ClientRef:              r.ClientRef,
			Description:            r.Description,
			Category:               r.Category,
			Amount:                 r.Amount,
			CurrencyCode:           r.CurrencyCode,
			BranchCashboxAccountID: r.CashboxAccountID,
			BranchShiftID:          shiftID,
			OccurredAt:             parseLocalTime(r.OccurredAt),
		})
		if err != nil {
			return rep, err
		}
		if err := a.store.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "expense", ClientRef: r.ClientRef, PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

func (a *App) resyncLoans(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "loans"}
	rows, err := a.store.ListAllLoans(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(rows)
	byRef := make(map[string]db.Loan, len(rows))
	refs := make([]string, 0, len(rows))
	for _, r := range rows {
		byRef[r.ClientRef] = r
		refs = append(refs, r.ClientRef)
	}

	missing, err := a.reconcileEntity(ctx, client, "loans", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("loans", rep.Central, rep.Local)
	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		r := byRef[ref]
		var shiftID *int64
		if r.ShiftID.Valid {
			shiftID = &r.ShiftID.Int64
		}
		body, err := json.Marshal(branchLoanPayload{
			ClientRef:              r.ClientRef,
			Description:            r.Description,
			CategoryName:           r.Category,
			Amount:                 r.Amount,
			CurrencyCode:           r.CurrencyCode,
			BranchCashboxAccountID: r.CashboxAccountID,
			BranchShiftID:          shiftID,
			OccurredAt:             parseLocalTime(r.OccurredAt),
		})
		if err != nil {
			return rep, err
		}
		if err := a.store.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "loan", ClientRef: r.ClientRef, PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

// emitResyncProgress feeds the settings-page progress line. done/total are
// best-effort ("N of M clients"); the frontend just shows them.
func (a *App) emitResyncProgress(stage string, done, total int) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "resync_progress", map[string]any{
			"stage": stage, "done": done, "total": total,
		})
	default:
	}
}
