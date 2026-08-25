package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// getOrCreateInvoiceClientRef returns the stable client_ref for a local
// invoice id, generating and persisting one on first use. A return/exchange
// must be able to reference its original sale by client_ref even if that
// sale was never itself queued for sync (e.g. sync was turned on later) —
// this guarantees any invoice id always resolves to the same ref no matter
// which code path asks first.
func getOrCreateInvoiceClientRef(ctx context.Context, q *db.Queries, invoiceID int64) (string, error) {
	ref, err := q.GetInvoiceClientRef(ctx, invoiceID)
	if err == nil {
		return ref, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	ref = uuid.NewString()
	if err := q.CreateInvoiceClientRef(ctx, db.CreateInvoiceClientRefParams{
		InvoiceID: invoiceID,
		ClientRef: ref,
	}); err != nil {
		return "", err
	}
	return ref, nil
}

// branchInvoicePayload is the exact JSON body POSTed to kashi's
// /branch/sales_invoices and /branch/return_invoices (see api/branch_sync.go
// in the kashi repo — branchInvoiceRequest there must stay in sync with this
// shape). Every ID here is this install's own local SQLite id, meaningless
// to kashi's schema — kashi stores them as opaque traceability data, not
// foreign keys, until branch-scoped identity sync exists.
type branchInvoicePayload struct {
	ClientRef              string `json:"client_ref"`
	BranchInvoiceCode      string `json:"branch_invoice_code"`
	BranchCashboxAccountID int64  `json:"branch_cashbox_account_id"`
	BranchShiftID          int64  `json:"branch_shift_id"`
	BranchInventoryID      int64  `json:"branch_inventory_id"`
	BranchClientID         int64  `json:"branch_client_id"`
	Discount               int16  `json:"discount"`
	GrandTotal             int64  `json:"grand_total"`
	Subtotal               int64  `json:"sub_total"`
	DiscountedTotal        int64  `json:"discounted_total"`
	// Whatever this install already computed and applied locally (see
	// AddClientLoyaltyPoints in tx_invoice.go) — kashi records this as
	// reported, it must never be re-derived from GrandTotal centrally.
	LoyaltyPointsDelta int64                      `json:"loyalty_points_delta"`
	OccurredAt         time.Time                  `json:"occurred_at"`
	Items              []branchInvoicePayloadItem `json:"items"`
	RelatedClientRef   string                     `json:"related_client_ref,omitempty"`
}

type branchInvoicePayloadItem struct {
	BranchProductID int64 `json:"branch_product_id"`
	UnitPrice       int64 `json:"unit_price"`
	LineTotal       int64 `json:"line_total"`
	Discount        int16 `json:"discount"`
	Quantity        int64 `json:"quantity"`
}

// branchExpensePayload is the exact JSON body POSTed to kashi's
// /branch/expenses (see api/branch_expense.go in the kashi repo —
// branchExpenseRequest there must stay in sync with this shape).
// BranchCashboxAccountID/BranchShiftID are this install's own local ids,
// meaningless to kashi's schema — kashi stores them as opaque traceability
// data, not foreign keys, same reasoning as branchInvoicePayload above.
type branchExpensePayload struct {
	ClientRef              string    `json:"client_ref"`
	Description            string    `json:"description"`
	Category               string    `json:"category"`
	Amount                 int64     `json:"amount"`
	CurrencyCode           string    `json:"currency_code"`
	BranchCashboxAccountID int64     `json:"branch_cashbox_account_id"`
	BranchShiftID          *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt             time.Time `json:"occurred_at"`
}

// queueBranchExpenseSync is queueBranchInvoiceSync's counterpart for
// expenses — same opt-in-if-configured, same-transaction-as-the-write
// behavior.
func queueBranchExpenseSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchExpensePayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "expense",
		ClientRef:   payload.ClientRef,
		PayloadJson: string(body),
	})
	return err
}

// branchLoanPayload is the exact JSON body POSTed to kashi's /branch/loans
// (see api/branch_loan.go in the kashi repo — branchLoanRequest there must
// stay in sync with this shape). BranchCashboxAccountID/BranchShiftID are
// this install's own local ids, meaningless to kashi's schema, same
// reasoning as branchExpensePayload above. CategoryName is a lender-source
// name (e.g. "Owner"/"External") -- kashi resolves it to its own
// loan_categories row, this install never fetches or caches that list.
type branchLoanPayload struct {
	ClientRef              string    `json:"client_ref"`
	Description            string    `json:"description"`
	CategoryName           string    `json:"category_name"`
	Amount                 int64     `json:"amount"`
	CurrencyCode           string    `json:"currency_code"`
	BranchCashboxAccountID int64     `json:"branch_cashbox_account_id"`
	BranchShiftID          *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt             time.Time `json:"occurred_at"`
}

// queueBranchLoanSync is queueBranchExpenseSync's counterpart for loans --
// same opt-in-if-configured, same-transaction-as-the-write behavior.
func queueBranchLoanSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchLoanPayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "loan",
		ClientRef:   payload.ClientRef,
		PayloadJson: string(body),
	})
	return err
}

// queueBranchInvoiceSync inserts a sync_outbox row using the same *db.Queries
// (and therefore the same transaction) as the invoice write that produced
// it, so a crash between committing the invoice and queueing it for sync
// can never happen — they commit together or not at all. It's a no-op if
// this install hasn't been configured to sync (empty central_api_url):
// sync is opt-in per branch, and an unconfigured install must behave
// exactly as it does today, indefinitely.
//
// payload.ClientRef must already be set by the caller — via
// getOrCreateInvoiceClientRef — not generated here, so the same invoice
// always gets the same ref regardless of which code path queues it.
func queueBranchInvoiceSync(ctx context.Context, q *db.Queries, centralAPIURL, entityType string, payload branchInvoicePayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  entityType,
		ClientRef:   payload.ClientRef,
		PayloadJson: string(body),
	})
	return err
}
