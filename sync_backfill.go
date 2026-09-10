package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	// Own the database for the duration: tell the 15s tick to stand down,
	// and hold syncPullMu so the WS client's catch-up pulls can't run
	// either. SQLite has one writer; without this, the tick's outbox drain
	// racing this run's writes is what trips "database is locked (517)".
	a.resyncActive.Store(true)
	defer a.resyncActive.Store(false)
	a.syncPullMu.Lock()
	defer a.syncPullMu.Unlock()

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
		if err := retryBusy(func() error { return a.store.ClearSyncCursors(ctx) }); err != nil {
			return report, fmt.Errorf("clear sync cursors: %w", err)
		}
		a.emitResyncProgress("pull", 0, 0)
		a.runSyncPullLocked(ctx, client)
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

	// 4c. Shift closes, visitor events, attendance -- all keyed by an
	//     on-row / derivable client_ref, all covered by reconcile.
	for _, fn := range []func(context.Context, *centralClient, ResyncOptions) (ResyncEntityReport, error){
		a.resyncShiftCloses, a.resyncVisitorEvents, a.resyncAttendanceEvents, a.resyncAttendanceChanges,
	} {
		rep, err := fn(ctx, client, opts)
		if err != nil {
			return report, fmt.Errorf("resync %s: %w", rep.Entity, err)
		}
		report.Entities = append(report.Entities, rep)
	}

	// 5. Everything else already in the outbox: re-arm rows a previous
	//    server confirmed (or that hard-failed) so they re-push to this one.
	rearmed := 0
	if !opts.DryRun {
		var n int64
		err := retryBusy(func() error {
			var e error
			n, e = a.store.RequeueSyncOutboxConfirmedElsewhere(ctx, client.baseURL)
			return e
		})
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
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
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
	if err := a.resyncTx(ctx, func(q *db.Queries) error {
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
		if err := a.resyncTx(ctx, func(q *db.Queries) error {
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
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
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
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "loan", ClientRef: r.ClientRef, PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

func (a *App) resyncShiftCloses(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "shift_closes"}
	shifts, err := a.store.ListClosedShifts(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(shifts)
	byRef := make(map[string]db.Shift, len(shifts))
	refs := make([]string, 0, len(shifts))
	for _, sh := range shifts {
		ref := fmt.Sprintf("shift-%d", sh.ID)
		byRef[ref] = sh
		refs = append(refs, ref)
	}

	missing, err := a.reconcileEntity(ctx, client, "shift_closes", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("shift_closes", rep.Central, rep.Local)
	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		sh := byRef[ref]
		name := ""
		if sh.ClosedBySalespersonID != 0 {
			if sp, err := a.store.GetSalespersonByID(ctx, sh.ClosedBySalespersonID); err == nil {
				name = sp.FullName
			}
		}
		body, err := json.Marshal(branchShiftClosePayload{
			ShiftRef:          ref,
			BranchShiftID:     sh.ID,
			OpenedAt:          parseLocalTime(sh.CreatedAt),
			ClosedAt:          parseLocalTime(sh.ClosedAt),
			ClosingPersonName: name,
			OpeningFloatUsd:   sh.OpeningFloatUsd,
			OpeningFloatLbp:   sh.OpeningFloatLbp,
			ExpectedUsd:       sh.ExpectedUsd,
			ExpectedLbp:       sh.ExpectedLbp,
			ExpectedVisa:      sh.ExpectedVisa,
			ExpectedWhish:     sh.ExpectedWhish,
			CountedUsd:        sh.CountedUsd,
			CountedLbp:        sh.CountedLbp,
			CountedVisa:       sh.CountedVisa,
			CountedWhish:      sh.CountedWhish,
			OccurredAt:        parseLocalTime(sh.ClosedAt),
		})
		if err != nil {
			return rep, err
		}
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "shift_close", ClientRef: ref, PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

func (a *App) resyncVisitorEvents(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "visitor_events"}
	rows, err := a.store.ListAllVisitorEvents(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(rows)
	byRef := make(map[string]db.ListAllVisitorEventsRow, len(rows))
	refs := make([]string, 0, len(rows))
	for _, r := range rows {
		byRef[r.ClientRef] = r
		refs = append(refs, r.ClientRef)
	}

	missing, err := a.reconcileEntity(ctx, client, "visitor_events", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("visitor_events", rep.Central, rep.Local)
	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		r := byRef[ref]
		body, err := json.Marshal(branchVisitorEventPayload{
			Ref: r.ClientRef, Day: r.Day, Direction: r.Direction,
			OccurredAt: parseLocalTime(r.OccurredAt),
		})
		if err != nil {
			return rep, err
		}
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "visitor_event", ClientRef: r.ClientRef, PayloadJson: string(body),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

func (a *App) resyncAttendanceEvents(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "attendance_events"}
	rows, err := a.store.ListAllSalespersonAttendanceEvents(ctx)
	if err != nil {
		return rep, err
	}
	rep.Local = len(rows)
	byRef := make(map[string]db.SalespersonAttendanceEvent, len(rows))
	refs := make([]string, 0, len(rows))
	for _, e := range rows {
		ref := e.DeviceID + "|" + e.EventID
		byRef[ref] = e
		refs = append(refs, ref)
	}

	missing, err := a.reconcileEntity(ctx, client, "attendance_events", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("attendance_events", rep.Central, rep.Local)
	if opts.DryRun || len(missing) == 0 {
		return rep, nil
	}

	// One batch payload / one outbox row for all missing punches, same as
	// the live device-import path; kashi upserts each row on (branch_id, ref).
	events := make([]branchAttendanceEvent, 0, len(missing))
	for _, ref := range missing {
		e := byRef[ref]
		events = append(events, branchAttendanceEvent{
			Ref:              ref,
			SalespersonName:  e.SalespersonName,
			AttendanceUserID: e.AttendanceUserID,
			EventDate:        e.EventDate,
			EventTime:        e.EventTime,
			EventAt:          e.EventAt,
			Type:             e.Type,
			Status:           e.Status,
		})
	}
	body, err := json.Marshal(branchAttendanceEventsPayload{Events: events})
	if err != nil {
		return rep, err
	}
	if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
		EntityType: "attendance_events", ClientRef: "attendance-events-resync", PayloadJson: string(body),
	}); err != nil {
		return rep, err
	}
	rep.Queued = len(events)
	return rep, nil
}

func (a *App) resyncAttendanceChanges(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "attendance_changes"}
	rows, err := a.store.ListAllAttendanceChangeRequests(ctx)
	if err != nil {
		return rep, err
	}

	// Each request produces a "complaint" event, plus a "review" event once
	// a manager has acted (reviewed_at set). Build both refs and their
	// payloads, then reconcile the whole set.
	type pending struct {
		ref  string
		body []byte
	}
	var candidates []pending
	nameFor := func(id int64) string {
		if id == 0 {
			return ""
		}
		if sp, err := a.store.GetSalespersonByID(ctx, id); err == nil {
			return sp.FullName
		}
		return ""
	}
	for _, r := range rows {
		name := nameFor(r.SalespersonID)
		complaintRef := fmt.Sprintf("acr-%d-complaint", r.ID)
		cb, err := json.Marshal(branchAttendanceChangePayload{
			Ref: complaintRef, Kind: "complaint", SalespersonName: name,
			AttendanceDate: r.AttendanceDate, OriginalTime: r.OriginalTime,
			RequestedTime: r.RequestedTime, RequestedType: r.RequestedType,
			Action: r.Action, Status: "pending", Note: r.Note,
			OccurredAt: parseLocalTime(r.CreatedAt),
		})
		if err != nil {
			return rep, err
		}
		candidates = append(candidates, pending{complaintRef, cb})

		if strings.TrimSpace(r.ReviewedAt) != "" {
			reviewRef := fmt.Sprintf("acr-%d-review", r.ID)
			note := "rejected"
			if r.Status == "approved" {
				note = strings.TrimSpace(fmt.Sprintf("%s -> %s", r.ChangedFrom, r.ChangedTo))
			}
			rb, err := json.Marshal(branchAttendanceChangePayload{
				Ref: reviewRef, Kind: "approval", SalespersonName: name,
				AttendanceDate: r.AttendanceDate, OriginalTime: r.OriginalTime,
				RequestedTime: r.RequestedTime, RequestedType: r.RequestedType,
				Action: r.Action, Status: r.Status, Note: note, Actor: "manager approval",
				OccurredAt: parseLocalTime(r.ReviewedAt),
			})
			if err != nil {
				return rep, err
			}
			candidates = append(candidates, pending{reviewRef, rb})
		}
	}
	rep.Local = len(candidates)

	byRef := make(map[string][]byte, len(candidates))
	refs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		byRef[c.ref] = c.body
		refs = append(refs, c.ref)
	}

	missing, err := a.reconcileEntity(ctx, client, "attendance_changes", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("attendance_changes", rep.Central, rep.Local)
	if opts.DryRun {
		return rep, nil
	}

	for _, ref := range missing {
		if err := a.upsertOutbox(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "attendance_change", ClientRef: ref, PayloadJson: string(byRef[ref]),
		}); err != nil {
			return rep, err
		}
		rep.Queued++
	}
	return rep, nil
}

// isDatabaseBusy reports whether err is a transient SQLite lock/busy
// condition worth retrying. modernc's driver surfaces these as text; the
// codes are SQLITE_BUSY (5), SQLITE_LOCKED (6) and SQLITE_BUSY_SNAPSHOT
// (517, which busy_timeout does NOT wait out on its own).
func isDatabaseBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "(517)") || strings.Contains(msg, "(5)") ||
		strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "SQLITE_LOCKED")
}

// retryBusy runs fn, retrying a handful of times with a short backoff while
// it fails with a transient lock. ResyncAll already tells the sync tick to
// stand down, but a foreground sale rung up mid-run is still a second
// writer, and one connection losing a WAL snapshot race must not abort a
// whole "sync everything".
func retryBusy(fn func() error) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		if err = fn(); !isDatabaseBusy(err) {
			return err
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return err
}

func (a *App) resyncTx(ctx context.Context, fn func(*db.Queries) error) error {
	return retryBusy(func() error { return a.store.ExecTx(ctx, fn) })
}

func (a *App) upsertOutbox(ctx context.Context, params db.UpsertSyncOutboxEntryParams) error {
	return retryBusy(func() error { return a.store.UpsertSyncOutboxEntry(ctx, params) })
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
