package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

const sqliteTimeLayout = "2006-01-02 15:04:05"

func formatHistoryTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(sqliteTimeLayout)
}

// PullBranchHistory imports this branch's own historical invoices, expenses,
// loans and shift closes from kashi -- for re-provisioning a branch that
// already has real sales history there onto a fresh, empty local database
// (see CreateBranch/SwitchBranch in branches.go for the everyday multi-
// branch flow this complements). It inserts directly into the local tables,
// seeding kashi's own client_ref on every row, and never touches
// sync_outbox -- these rows are already on kashi, so nothing here should
// ever be queued to push back.
//
// Refuses to run unless every affected local table is empty: this is a
// one-time recovery action for a freshly created branch, not an ongoing
// sync path, and running it against a till with real local data would risk
// duplicating or misattributing that data. It is a best-effort
// reconstruction, not a perfect restore -- see the doc comments on
// importBranchExpenseHistory/importBranchLoanHistory (payment account
// identity) and resolveHistoricalProduct (line items with no barcode/uid
// ever recorded) for the two known, disclosed limitations.
func (a *App) PullBranchHistory() (map[string]interface{}, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return nil, fmt.Errorf("only an admin can import branch history")
	}

	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	if settings.CentralApiUrl == "" || settings.BranchCode == "" || settings.ApiKey == "" {
		return nil, fmt.Errorf("central sync is not fully configured (server URL, branch code and API key are all required)")
	}

	if empty, err := a.localHistoryIsEmpty(); err != nil {
		return nil, err
	} else if !empty {
		return nil, fmt.Errorf("this branch already has local invoices, expenses, loans, or shifts -- history import only runs on a freshly created branch")
	}

	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	if !client.enabled() {
		return nil, fmt.Errorf("central sync is not fully configured")
	}

	// Guarantee the full product/client/price-list catalog is present before
	// resolving any historical line item against it. Same primitive
	// ResyncAll's own step 1 uses, without ResyncAll's push-up and
	// deactivation phases -- neither applies here: there's nothing local yet
	// to push, and nothing local yet to deactivate against.
	if err := retryBusy(func() error { return a.store.ClearSyncCursors(a.ctx) }); err != nil {
		return nil, fmt.Errorf("clear sync cursors: %w", err)
	}
	a.runSyncPullLocked(a.ctx, client)

	imp := &branchHistoryImporter{
		app:               a,
		client:            client,
		shiftIDByBranch:   map[int64]int64{},
		salespersonByName: map[string]int64{},
		cashboxByName:     map[string]int64{},
		invoiceIDByRef:    map[string]int64{},
	}

	shiftCount, err := imp.importShifts(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("import shift history: %w", err)
	}
	invoiceCount, unresolvedItems, err := imp.importInvoices(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("import invoice history: %w", err)
	}
	expenseCount, err := imp.importExpenses(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("import expense history: %w", err)
	}
	loanCount, err := imp.importLoans(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("import loan history: %w", err)
	}

	return map[string]interface{}{
		"shiftCloses":     shiftCount,
		"invoices":        invoiceCount,
		"unresolvedItems": unresolvedItems,
		"expenses":        expenseCount,
		"loans":           loanCount,
	}, nil
}

func (a *App) localHistoryIsEmpty() (bool, error) {
	if n, err := a.store.CountInvoices(a.ctx); err != nil {
		return false, err
	} else if n > 0 {
		return false, nil
	}
	if n, err := a.store.CountExpensesAll(a.ctx); err != nil {
		return false, err
	} else if n > 0 {
		return false, nil
	}
	if n, err := a.store.CountLoansAll(a.ctx); err != nil {
		return false, err
	} else if n > 0 {
		return false, nil
	}
	if n, err := a.store.CountShiftsAll(a.ctx); err != nil {
		return false, err
	} else if n > 0 {
		return false, nil
	}
	return true, nil
}

// branchHistoryImporter holds the resolution caches shared across the four
// entity imports, so e.g. the salesperson/cashbox-account lookups only walk
// their local tables once for the whole run rather than once per row.
type branchHistoryImporter struct {
	app    *App
	client *centralClient

	// shiftIDByBranch maps kashi's branch_shift_id (the old till's own local
	// shifts.id, opaque to kashi -- see branch_shifts' table comment on the
	// kashi side) to the new local shifts.id created for it. Only ever
	// populated by importShifts, which must run first.
	shiftIDByBranch map[int64]int64

	salespersonByName map[string]int64
	cashboxByName     map[string]int64

	// invoiceIDByRef maps a sale's own client_ref to its new local invoice
	// id, so a return pulled later in the same walk (returns are always
	// pushed after the sale they reference, so this is populated by the
	// time a return needs it) can resolve return_invoices.sales_invoice_id.
	invoiceIDByRef map[string]int64

	placeholderProductID int64
	defaultInventoryID   int64
	defaultCashboxID     int64
	walkInClientID       int64
}

// syncBranchShiftHistoryRow mirrors kashi's api/branch_catchup.go
// "branch_shift_history" case (backed by the branch_shifts table there).
type syncBranchShiftHistoryRow struct {
	ClientRef         string    `json:"client_ref"`
	BranchShiftID     int64     `json:"branch_shift_id"`
	OpenedAt          time.Time `json:"opened_at"`
	ClosedAt          time.Time `json:"closed_at"`
	ClosingPersonName string    `json:"closing_person_name"`
	OpeningFloatUsd   int64     `json:"opening_float_usd"`
	OpeningFloatLbp   int64     `json:"opening_float_lbp"`
	ExpectedUsd       int64     `json:"expected_usd"`
	ExpectedLbp       int64     `json:"expected_lbp"`
	ExpectedVisa      int64     `json:"expected_visa"`
	ExpectedWhish     int64     `json:"expected_whish"`
	CountedUsd        int64     `json:"counted_usd"`
	CountedLbp        int64     `json:"counted_lbp"`
	CountedVisa       int64     `json:"counted_visa"`
	CountedWhish      int64     `json:"counted_whish"`
}

// importShifts must run before importInvoices/importExpenses/importLoans:
// it's the only import that populates shiftIDByBranch, which those three
// use to resolve the branch_shift_id each of their own rows carries.
func (imp *branchHistoryImporter) importShifts(ctx context.Context) (int, error) {
	a := imp.app
	count := 0
	pageCursor := ""
	for {
		changes, err := imp.client.FetchChanges(ctx, fetchChangesParams{
			Entity: "branch_shift_history", Cursor: pageCursor, Limit: syncPullPageLimit,
		})
		if err != nil {
			return 0, err
		}
		var rows []syncBranchShiftHistoryRow
		if err := json.Unmarshal(changes.Items, &rows); err != nil {
			return 0, fmt.Errorf("decode shift history: %w", err)
		}
		for _, r := range rows {
			openedAt := r.OpenedAt
			if openedAt.IsZero() {
				openedAt = r.ClosedAt
			}
			shift, err := a.store.CreateHistoricalShift(ctx, db.CreateHistoricalShiftParams{
				CreatedAt:       formatHistoryTime(openedAt),
				ClosedAt:        formatHistoryTime(r.ClosedAt),
				CountedUsd:      r.CountedUsd,
				CountedLbp:      r.CountedLbp,
				CountedVisa:     r.CountedVisa,
				CountedWhish:    r.CountedWhish,
				ExpectedUsd:     r.ExpectedUsd,
				ExpectedLbp:     r.ExpectedLbp,
				ExpectedVisa:    r.ExpectedVisa,
				ExpectedWhish:   r.ExpectedWhish,
				OpeningFloatUsd: r.OpeningFloatUsd,
				OpeningFloatLbp: r.OpeningFloatLbp,
				ClientRef:       sql.NullString{String: r.ClientRef, Valid: r.ClientRef != ""},
			})
			if err != nil {
				return 0, fmt.Errorf("create historical shift %s: %w", r.ClientRef, err)
			}
			imp.shiftIDByBranch[r.BranchShiftID] = shift.ID
			count++
		}
		if changes.NextCursor == "" {
			break
		}
		pageCursor = changes.NextCursor
	}
	return count, nil
}

// resolveSalesperson finds or creates a local salesperson by name.
// invoices.salesperson_id/invoice_products.salesperson_id are plain bigint
// columns with no foreign-key constraint (see migration 000022), so 0 (no
// match, no name given) is always a safe fallback, not an error.
func (imp *branchHistoryImporter) resolveSalesperson(ctx context.Context, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	if id, ok := imp.salespersonByName[name]; ok {
		return id, nil
	}
	if len(imp.salespersonByName) == 0 {
		existing, err := imp.app.store.ListSalespersons(ctx)
		if err != nil {
			return 0, err
		}
		for _, s := range existing {
			imp.salespersonByName[s.FullName] = s.ID
		}
		if id, ok := imp.salespersonByName[name]; ok {
			return id, nil
		}
	}
	created, err := imp.app.store.CreateSalesperson(ctx, name)
	if err != nil {
		return 0, err
	}
	imp.salespersonByName[name] = created.ID
	return created.ID, nil
}

// resolveCashboxAccount finds a local cashbox account by its display name --
// cashbox accounts sync down from kashi by name (see ListCashboxAccountsForBranchSync
// on the kashi side), so any account this branch has ever used should
// already be present locally by the time this runs (after the full catalog
// pull PullBranchHistory does first). Falls back to defaultCashboxID
// (resolved once, lazily) when the name is blank or no longer matches any
// local account -- e.g. it was renamed or deactivated centrally since.
func (imp *branchHistoryImporter) resolveCashboxAccount(ctx context.Context, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name != "" {
		if id, ok := imp.cashboxByName[name]; ok {
			return id, nil
		}
		if account, err := imp.app.store.GetCashboxAccountByName(ctx, name); err == nil {
			imp.cashboxByName[name] = account.ID
			return account.ID, nil
		}
	}
	if imp.defaultCashboxID != 0 {
		return imp.defaultCashboxID, nil
	}
	accounts, err := imp.app.store.ListCashboxAccounts(ctx)
	if err != nil {
		return 0, err
	}
	if len(accounts) == 0 {
		return 0, fmt.Errorf("no local cashbox accounts to attach a historical record to -- run the full sync pull first")
	}
	imp.defaultCashboxID = accounts[0].ID
	return imp.defaultCashboxID, nil
}

func (imp *branchHistoryImporter) resolveInventory(ctx context.Context) (int64, error) {
	if imp.defaultInventoryID != 0 {
		return imp.defaultInventoryID, nil
	}
	settings, err := imp.app.store.GetSettings(ctx)
	if err == nil && settings.SalesInventoryID > 0 {
		if _, err := imp.app.store.GetInventoryByID(ctx, settings.SalesInventoryID); err == nil {
			imp.defaultInventoryID = settings.SalesInventoryID
			return imp.defaultInventoryID, nil
		}
	}
	inventories, err := imp.app.store.ListInventories(ctx)
	if err != nil {
		return 0, err
	}
	if len(inventories) == 0 {
		return 0, fmt.Errorf("no local inventory to attach a historical invoice to")
	}
	imp.defaultInventoryID = inventories[0].ID
	return imp.defaultInventoryID, nil
}

func (imp *branchHistoryImporter) resolveClient(ctx context.Context, clientUID string) (int64, error) {
	if clientUID != "" {
		if client, err := imp.app.store.GetClientByUID(ctx, sql.NullString{String: clientUID, Valid: true}); err == nil {
			return client.ID, nil
		}
	}
	if imp.walkInClientID != 0 {
		return imp.walkInClientID, nil
	}
	clients, err := imp.app.store.ListClients(ctx)
	if err != nil {
		return 0, err
	}
	for _, c := range clients {
		if c.Name == "Walk-in" {
			imp.walkInClientID = c.ID
			return c.ID, nil
		}
	}
	if len(clients) > 0 {
		imp.walkInClientID = clients[0].ID
		return clients[0].ID, nil
	}
	return 0, fmt.Errorf("no local client to attach a historical invoice to")
}

// resolveHistoricalProduct resolves a pulled line item to a local product,
// preferring ProductUID (the precise match) over Barcode. Falls back to a
// once-created placeholder product only when NEITHER is resolvable --
// expected to be rare in practice: kashi never deletes a product used in
// any invoice (only deactivates it), and PullBranchHistory pulls the
// complete catalog, active and deactivated, before importing any invoice.
// The only rows that can still miss are ones pushed before kashi tracked
// product_uid at all and whose barcode never resolved to a central variant
// at push time either -- see the kashi repo's tx_branch_invoice.go.
func (imp *branchHistoryImporter) resolveHistoricalProduct(ctx context.Context, item syncBranchInvoiceHistoryItem) (int64, bool, error) {
	if item.ProductUID != "" {
		if p, err := imp.app.store.GetProductByUID(ctx, sql.NullString{String: item.ProductUID, Valid: true}); err == nil {
			return p.ID, true, nil
		}
	}
	if item.Barcode != "" {
		if p, err := imp.app.store.GetProductByBarcode(ctx, item.Barcode); err == nil {
			return p.ID, true, nil
		}
	}
	if imp.placeholderProductID != 0 {
		return imp.placeholderProductID, false, nil
	}
	const placeholderBarcode = "HISTORY-UNRESOLVED"
	if p, err := imp.app.store.GetProductByBarcode(ctx, placeholderBarcode); err == nil {
		imp.placeholderProductID = p.ID
		return p.ID, false, nil
	}
	p, err := imp.app.store.CreateProduct(ctx, db.CreateProductParams{
		Barcode:      placeholderBarcode,
		Code:         placeholderBarcode,
		Name:         "Unresolved historical item",
		Description:  "Placeholder for a historical line item with no product this branch's local catalog could resolve -- see PullBranchHistory.",
		IsActive:     0,
		CurrencyCode: "USD",
	})
	if err != nil {
		return 0, false, err
	}
	imp.placeholderProductID = p.ID
	return p.ID, false, nil
}

// syncBranchInvoiceHistoryItem/Payment/Row mirror kashi's
// api/branch_history_sync.go response shapes exactly.
type syncBranchInvoiceHistoryItem struct {
	Barcode    string `json:"barcode"`
	ProductUID string `json:"product_uid,omitempty"`
	UnitPrice  int64  `json:"unit_price"`
	LineTotal  int64  `json:"line_total"`
	Discount   int64  `json:"discount"`
	Quantity   int64  `json:"quantity"`
}

type syncBranchInvoiceHistoryPayment struct {
	AccountName string `json:"account_name"`
	Amount      int64  `json:"amount"`
	CouponCode  string `json:"coupon_code,omitempty"`
}

type syncBranchInvoiceHistoryRow struct {
	ClientRef              string                            `json:"client_ref"`
	Kind                   string                            `json:"kind"`
	BranchInvoiceCode      string                            `json:"branch_invoice_code"`
	RelatedClientRef       string                            `json:"related_client_ref,omitempty"`
	BranchShiftID          int64                             `json:"branch_shift_id"`
	BranchCashboxAccountID int64                             `json:"branch_cashbox_account_id"`
	ClientUID              string                            `json:"client_uid,omitempty"`
	Discount               int64                             `json:"discount"`
	Subtotal               int64                             `json:"subtotal"`
	DiscountedTotal        int64                             `json:"discounted_total"`
	GrandTotal             int64                             `json:"grand_total"`
	OccurredAt             time.Time                         `json:"occurred_at"`
	SalespersonName        string                            `json:"salesperson_name"`
	Historical             bool                              `json:"historical"`
	Items                  []syncBranchInvoiceHistoryItem    `json:"items"`
	Payments               []syncBranchInvoiceHistoryPayment `json:"payments"`
}

// invoiceCodeIndex parses the trailing numeric segment off a branch invoice
// code like "SA/CN/2026/00001" for invoices.invoice_index. This is
// deliberately independent of the live numbering sequence
// (IncrementInvoicesIndex / invoice_indexes) -- a historical import never
// touches that counter, so an arbitrary or non-sequential index here can
// never collide with or corrupt future live invoice numbering.
func invoiceCodeIndex(code string) int64 {
	parts := strings.Split(code, "/")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (imp *branchHistoryImporter) importInvoices(ctx context.Context) (int, int, error) {
	count := 0
	unresolvedItems := 0
	pageCursor := ""
	for {
		changes, err := imp.client.FetchChanges(ctx, fetchChangesParams{
			Entity: "branch_invoice_history", Cursor: pageCursor, Limit: syncPullPageLimit,
		})
		if err != nil {
			return count, unresolvedItems, err
		}
		var rows []syncBranchInvoiceHistoryRow
		if err := json.Unmarshal(changes.Items, &rows); err != nil {
			return count, unresolvedItems, fmt.Errorf("decode invoice history: %w", err)
		}
		for _, r := range rows {
			n, err := imp.importOneInvoice(ctx, r)
			if err != nil {
				return count, unresolvedItems, fmt.Errorf("import invoice %s: %w", r.ClientRef, err)
			}
			unresolvedItems += n
			count++
		}
		if changes.NextCursor == "" {
			break
		}
		pageCursor = changes.NextCursor
	}
	return count, unresolvedItems, nil
}

func (imp *branchHistoryImporter) importOneInvoice(ctx context.Context, r syncBranchInvoiceHistoryRow) (int, error) {
	a := imp.app

	salespersonID, err := imp.resolveSalesperson(ctx, r.SalespersonName)
	if err != nil {
		return 0, err
	}
	inventoryID, err := imp.resolveInventory(ctx)
	if err != nil {
		return 0, err
	}
	clientID, err := imp.resolveClient(ctx, r.ClientUID)
	if err != nil {
		return 0, err
	}
	shiftID, ok := imp.shiftIDByBranch[r.BranchShiftID]
	if !ok {
		// This sale's own shift wasn't part of this branch's pulled shift
		// history (e.g. that shift was never closed, or its close never
		// synced) -- attach it to a single lazily-created catch-all shift
		// for the whole import run instead of failing the invoice outright.
		shiftID, err = imp.fallbackShiftID(ctx)
		if err != nil {
			return 0, err
		}
	}
	cashboxID, err := imp.resolveCashboxAccount(ctx, "")
	if len(r.Payments) > 0 {
		if id, err2 := imp.resolveCashboxAccount(ctx, r.Payments[0].AccountName); err2 == nil {
			cashboxID = id
			err = nil
		}
	}
	if err != nil {
		return 0, err
	}

	invoice, err := a.store.CreateHistoricalInvoice(ctx, db.CreateHistoricalInvoiceParams{
		CashboxID:       cashboxID,
		ShiftID:         shiftID,
		InvoiceCode:     r.BranchInvoiceCode,
		InvoiceIndex:    invoiceCodeIndex(r.BranchInvoiceCode),
		Year:            int64(r.OccurredAt.Year()),
		ClientID:        clientID,
		InventoryID:     inventoryID,
		Discount:        r.Discount,
		Subtotal:        r.Subtotal,
		DiscountedTotal: r.DiscountedTotal,
		GrandTotal:      r.GrandTotal,
		SalespersonID:   salespersonID,
		Notes:           "",
		CreatedAt:       formatHistoryTime(r.OccurredAt),
	})
	if err != nil {
		return 0, fmt.Errorf("create invoice: %w", err)
	}

	if err := a.store.CreateInvoiceClientRef(ctx, db.CreateInvoiceClientRefParams{
		InvoiceID: invoice.ID, ClientRef: r.ClientRef,
	}); err != nil {
		return 0, fmt.Errorf("create invoice client ref: %w", err)
	}
	imp.invoiceIDByRef[r.ClientRef] = invoice.ID

	if r.Kind == "return" {
		salesInvoiceID := imp.invoiceIDByRef[r.RelatedClientRef]
		if salesInvoiceID == 0 {
			// The original sale wasn't part of this pull (predates the
			// window, or belongs to a different branch's history entirely,
			// which should never happen since this is scoped to one
			// branch) -- point it at itself rather than leaving an invalid
			// reference; the return's own totals/items/payments are still
			// fully recorded either way.
			salesInvoiceID = invoice.ID
			log.Printf("historical return %s: original sale %s not found, self-referencing", r.ClientRef, r.RelatedClientRef)
		}
		if err := a.store.CreateReturnInvoice(ctx, db.CreateReturnInvoiceParams{
			InvoiceID: invoice.ID, SalesInvoiceID: salesInvoiceID,
		}); err != nil {
			return 0, fmt.Errorf("create return invoice: %w", err)
		}
	} else {
		if err := a.store.CreateSalesInvoice(ctx, invoice.ID); err != nil {
			return 0, fmt.Errorf("create sales invoice: %w", err)
		}
	}

	unresolved := 0
	for _, item := range r.Items {
		productID, resolved, err := imp.resolveHistoricalProduct(ctx, item)
		if err != nil {
			return unresolved, fmt.Errorf("resolve product: %w", err)
		}
		if !resolved {
			unresolved++
		}
		if err := a.store.CreateInvoiceProduct(ctx, db.CreateInvoiceProductParams{
			InvoiceID:     invoice.ID,
			ProductID:     productID,
			UnitPrice:     item.UnitPrice,
			LineTotal:     item.LineTotal,
			Discount:      item.Discount,
			Quantity:      item.Quantity,
			SalespersonID: salespersonID,
		}); err != nil {
			return unresolved, fmt.Errorf("create invoice product: %w", err)
		}
	}

	for _, payment := range r.Payments {
		accountID, err := imp.resolveCashboxAccount(ctx, payment.AccountName)
		if err != nil {
			return unresolved, err
		}
		if err := a.store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
			InvoiceID: invoice.ID, CashboxID: accountID, Amount: payment.Amount,
		}); err != nil {
			return unresolved, fmt.Errorf("create invoice payment: %w", err)
		}
	}

	return unresolved, nil
}

// fallbackShiftID lazily creates one placeholder shift for the whole import
// run, for any invoice/expense/loan whose own branch_shift_id can't be
// resolved from the pulled shift history. Created already closed (nothing
// should ever open against it), with zeroed counted/expected fields -- it
// exists purely so records that must have *a* shift attach to a real,
// honestly-named one instead of failing the whole import.
func (imp *branchHistoryImporter) fallbackShiftID(ctx context.Context) (int64, error) {
	const fallbackRef = "history-import-fallback-shift"
	if id, ok := imp.shiftIDByBranch[-1]; ok {
		return id, nil
	}
	if existing, err := imp.app.store.GetShiftByClientRef(ctx, sql.NullString{String: fallbackRef, Valid: true}); err == nil {
		imp.shiftIDByBranch[-1] = existing.ID
		return existing.ID, nil
	}
	now := time.Now()
	shift, err := imp.app.store.CreateHistoricalShift(ctx, db.CreateHistoricalShiftParams{
		CreatedAt: formatHistoryTime(now),
		ClosedAt:  formatHistoryTime(now),
		ClientRef: sql.NullString{String: fallbackRef, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	imp.shiftIDByBranch[-1] = shift.ID
	return shift.ID, nil
}

type syncBranchExpenseHistoryRow struct {
	ClientRef                string    `json:"client_ref"`
	Description              string    `json:"description"`
	Amount                   int64     `json:"amount"`
	CurrencyCode             string    `json:"currency_code"`
	BranchShiftID            *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt               time.Time `json:"occurred_at"`
	CategoryName             string    `json:"category_name"`
	BranchCashboxAccountName *string   `json:"branch_cashbox_account_name,omitempty"`
}

// importExpenses is a best-effort reconstruction, not a perfect restore:
// only expenses synced after kashi's migration 000102 (see that repo) carry
// branch_cashbox_account_name at all -- older rows only ever had an opaque,
// now-meaningless local id, so resolveCashboxAccount's blank-name fallback
// (any local account, right currency not even guaranteed) is genuinely the
// best available for those.
func (imp *branchHistoryImporter) importExpenses(ctx context.Context) (int, error) {
	a := imp.app
	count := 0
	pageCursor := ""
	for {
		changes, err := imp.client.FetchChanges(ctx, fetchChangesParams{
			Entity: "branch_expense_history", Cursor: pageCursor, Limit: syncPullPageLimit,
		})
		if err != nil {
			return count, err
		}
		var rows []syncBranchExpenseHistoryRow
		if err := json.Unmarshal(changes.Items, &rows); err != nil {
			return count, fmt.Errorf("decode expense history: %w", err)
		}
		for _, r := range rows {
			accountName := ""
			if r.BranchCashboxAccountName != nil {
				accountName = *r.BranchCashboxAccountName
			}
			accountID, err := imp.resolveCashboxAccount(ctx, accountName)
			if err != nil {
				return count, err
			}
			var shiftID sql.NullInt64
			if r.BranchShiftID != nil {
				if id, ok := imp.shiftIDByBranch[*r.BranchShiftID]; ok {
					shiftID = sql.NullInt64{Int64: id, Valid: true}
				}
			}
			if _, err := a.store.CreateHistoricalExpense(ctx, db.CreateHistoricalExpenseParams{
				ClientRef:        r.ClientRef,
				Description:      r.Description,
				Category:         r.CategoryName,
				Amount:           r.Amount,
				CurrencyCode:     r.CurrencyCode,
				CashboxAccountID: accountID,
				ShiftID:          shiftID,
				OccurredAt:       formatHistoryTime(r.OccurredAt),
				CreatedAt:        formatHistoryTime(r.OccurredAt),
			}); err != nil {
				return count, fmt.Errorf("create historical expense %s: %w", r.ClientRef, err)
			}
			count++
		}
		if changes.NextCursor == "" {
			break
		}
		pageCursor = changes.NextCursor
	}
	return count, nil
}

type syncBranchLoanHistoryRow struct {
	ClientRef                string    `json:"client_ref"`
	Description              string    `json:"description"`
	Amount                   int64     `json:"amount"`
	CurrencyCode             string    `json:"currency_code"`
	BranchShiftID            *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt               time.Time `json:"occurred_at"`
	CategoryName             string    `json:"category_name"`
	BranchCashboxAccountName *string   `json:"branch_cashbox_account_name,omitempty"`
}

// importLoans mirrors importExpenses exactly -- same best-effort account
// matching caveat.
func (imp *branchHistoryImporter) importLoans(ctx context.Context) (int, error) {
	a := imp.app
	count := 0
	pageCursor := ""
	for {
		changes, err := imp.client.FetchChanges(ctx, fetchChangesParams{
			Entity: "branch_loan_history", Cursor: pageCursor, Limit: syncPullPageLimit,
		})
		if err != nil {
			return count, err
		}
		var rows []syncBranchLoanHistoryRow
		if err := json.Unmarshal(changes.Items, &rows); err != nil {
			return count, fmt.Errorf("decode loan history: %w", err)
		}
		for _, r := range rows {
			accountName := ""
			if r.BranchCashboxAccountName != nil {
				accountName = *r.BranchCashboxAccountName
			}
			accountID, err := imp.resolveCashboxAccount(ctx, accountName)
			if err != nil {
				return count, err
			}
			var shiftID sql.NullInt64
			if r.BranchShiftID != nil {
				if id, ok := imp.shiftIDByBranch[*r.BranchShiftID]; ok {
					shiftID = sql.NullInt64{Int64: id, Valid: true}
				}
			}
			if _, err := a.store.CreateHistoricalLoan(ctx, db.CreateHistoricalLoanParams{
				ClientRef:        r.ClientRef,
				Description:      r.Description,
				Category:         r.CategoryName,
				Amount:           r.Amount,
				CurrencyCode:     r.CurrencyCode,
				CashboxAccountID: accountID,
				ShiftID:          shiftID,
				OccurredAt:       formatHistoryTime(r.OccurredAt),
				CreatedAt:        formatHistoryTime(r.OccurredAt),
			}); err != nil {
				return count, fmt.Errorf("create historical loan %s: %w", r.ClientRef, err)
			}
			count++
		}
		if changes.NextCursor == "" {
			break
		}
		pageCursor = changes.NextCursor
	}
	return count, nil
}
