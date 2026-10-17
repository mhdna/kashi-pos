package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// syncCentralAPIURL returns this install's configured central sync URL, or
// "" when there's no settings row yet (a bare test fixture, or a
// freshly-initialised database) — the same "sync is off" outcome as an
// empty central_api_url, so callers can branch on the string alone.
func (a *App) syncCentralAPIURL() (string, error) {
	settings, err := a.store.GetSettings(a.ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return settings.CentralApiUrl, nil
}

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
	// ClientUID is the buyer's shared identity; empty for a walk-in sale.
	ClientUID       string `json:"client_uid,omitempty"`
	Discount        int16  `json:"discount"`
	GrandTotal      int64  `json:"grand_total"`
	Subtotal        int64  `json:"sub_total"`
	DiscountedTotal int64  `json:"discounted_total"`
	// Whatever this install already computed and applied locally (see
	// AddClientLoyaltyPoints in tx_invoice.go) — kashi records this as
	// reported, it must never be re-derived from GrandTotal centrally.
	LoyaltyPointsDelta int64     `json:"loyalty_points_delta"`
	OccurredAt         time.Time `json:"occurred_at"`
	// Best-effort display name, not an id -- salespersons aren't a synced/
	// shared entity across the branch boundary the way products are, so
	// there's no branch-scoped id for kashi to store a foreign key against.
	// Empty when no salesperson was recorded for this invoice.
	SalespersonName string `json:"salesperson_name,omitempty"`
	// SalespersonID is kashi's id for that salesperson (salespersons.
	// remote_id), so a renamed salesperson stays one person on kashi's
	// reports. 0 (omitted) when they aren't one of kashi's.
	SalespersonID    int64                         `json:"salesperson_id,omitempty"`
	Payments         []branchInvoicePaymentPayload `json:"payments"`
	Items            []branchInvoicePayloadItem    `json:"items"`
	RelatedClientRef string                        `json:"related_client_ref,omitempty"`
	// Historical marks a one-time backfill of a pre-sync sale: kashi records
	// it in full but moves no central stock (see api/branch_sync.go). Never
	// set on a live checkout, so it stays omitted from those payloads.
	Historical bool `json:"historical,omitempty"`
	// Cancellation marks a return that cancels its sale outright (see
	// CancelInvoice): kashi stamps cancelled_at on both and leaves them out
	// of sales statistics. Only ever set on a return.
	Cancellation bool `json:"cancellation,omitempty"`
	// LbpTotal/LbpRate are the LBP total the till printed and the LBP per
	// USD it used (stampInvoiceLBPTotal), so kashi's reports show what was
	// actually sold in LBP. Omitted for invoices from before they existed.
	LbpTotal *int64 `json:"lbp_total,omitempty"`
	LbpRate  *int64 `json:"lbp_rate,omitempty"`
}

type branchInvoicePayloadItem struct {
	BranchProductID int64 `json:"branch_product_id"`
	// Barcode is this line's SKU barcode -- the identifier kashi uses to
	// resolve the line to a central product_variants row and decrement the
	// branch's central store inventory. BranchProductID stays for
	// traceability but doesn't resolve to anything central.
	Barcode string `json:"barcode"`
	// ProductUID is the line's shared variant identity; kashi prefers it
	// over Barcode and falls back to Barcode when it doesn't resolve.
	ProductUID string `json:"product_uid,omitempty"`
	UnitPrice  int64  `json:"unit_price"`
	LineTotal  int64  `json:"line_total"`
	Discount   int16  `json:"discount"`
	Quantity   int64  `json:"quantity"`
}

// clientUIDForSync returns the uid to report as an invoice's buyer: empty
// for the walk-in pseudo-client (never synced) or an unknown id.
func clientUIDForSync(ctx context.Context, q *db.Queries, clientID int64) string {
	c, err := q.GetClientByID(ctx, clientID)
	if err != nil || isWalkInClient(c) {
		return ""
	}
	return c.Uid.String
}

// branchInvoicePaymentPayload is one settlement line -- net amount paid
// through one payment account for this invoice (see invoice_payments,
// already computed at checkout: one row per account, payment minus change
// merged). AccountName is a display name, same "not a synced entity, so no
// id to send" reasoning as SalespersonName above.
type branchInvoicePaymentPayload struct {
	AccountName string `json:"account_name"`
	Amount      int64  `json:"amount"`
	// CouponCode is set only on the one payment line that redeemed a
	// coupon -- see GetInvoiceCouponPaymentByInvoiceID below. kashi uses
	// it to mark that code redeemed centrally (RedeemCoupon in its
	// tx_branch_invoice.go), which is what stops the same code being
	// redeemed again at a different branch.
	CouponCode string `json:"coupon_code,omitempty"`
}

// buildBranchInvoicePayments reads this invoice's already-persisted
// per-account settlement (see invoice_payments, written by
// recordExchangePayments/salesInvoiceTx) and resolves each account id to
// its display name for the sync payload.
func buildBranchInvoicePayments(ctx context.Context, q *db.Queries, invoiceID int64) ([]branchInvoicePaymentPayload, error) {
	rows, err := q.ListInvoicePayments(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("load invoice payments: %w", err)
	}

	// At most one coupon payment per invoice (invoice_coupon_payments.
	// invoice_id is that table's own primary key) -- "not found" just
	// means this invoice didn't pay with a coupon at all.
	var couponCode string
	if couponPayment, err := q.GetInvoiceCouponPaymentByInvoiceID(ctx, invoiceID); err == nil {
		couponCode = couponPayment.CouponCode
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load invoice coupon payment: %w", err)
	}

	payments := make([]branchInvoicePaymentPayload, 0, len(rows))
	for _, row := range rows {
		account, err := q.GetCashboxAccountByID(ctx, row.CashboxID)
		if err != nil {
			return nil, fmt.Errorf("load cashbox account %d: %w", row.CashboxID, err)
		}
		payment := branchInvoicePaymentPayload{
			AccountName: account.Name,
			Amount:      row.Amount,
		}
		if strings.EqualFold(account.Name, "On Coupon") {
			payment.CouponCode = couponCode
		}
		payments = append(payments, payment)
	}
	return payments, nil
}

// branchExpensePayload is the exact JSON body POSTed to kashi's
// /branch/expenses (see api/branch_expense.go in the kashi repo —
// branchExpenseRequest there must stay in sync with this shape).
// BranchCashboxAccountID/BranchShiftID are this install's own local ids,
// meaningless to kashi's schema — kashi stores them as opaque traceability
// data, not foreign keys, same reasoning as branchInvoicePayload above.
type branchExpensePayload struct {
	ClientRef              string `json:"client_ref"`
	Description            string `json:"description"`
	Category               string `json:"category"`
	Amount                 int64  `json:"amount"`
	CurrencyCode           string `json:"currency_code"`
	BranchCashboxAccountID int64  `json:"branch_cashbox_account_id"`
	// AccountName is the payment account's display name, same reasoning as
	// branchInvoicePaymentPayload.AccountName: unlike an id, it survives a
	// branch's local database being wiped and reprovisioned, which is what
	// lets a pulled-down historical expense be matched back to a real local
	// account by name instead of an opaque, no-longer-meaningful integer.
	AccountName   string    `json:"account_name"`
	BranchShiftID *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
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

// branchTransferRequestItemPayload is one line of stock this till proposes
// moving out of its own store inventory. ProductUID wins over Barcode when
// present, same preference order as branchInvoiceItemPayload's fields.
type branchTransferRequestItemPayload struct {
	Barcode    string `json:"barcode"`
	ProductUID string `json:"product_uid"`
	Quantity   int64  `json:"quantity"`
}

// branchTransferRequestPayload is the exact JSON body POSTed to kashi's
// /branch/transfer_requests (see api/branch_transfer.go in the kashi repo).
// ToBranchRef is another branch's code, as this install last pulled it
// down via the branches sync feed (see synced_branches, sync_pull_coupons.go).
type branchTransferRequestPayload struct {
	ClientRef   string                             `json:"client_ref"`
	ToBranchRef string                             `json:"to_branch_ref"`
	Note        string                             `json:"note"`
	Items       []branchTransferRequestItemPayload `json:"items"`
}

// queueBranchTransferRequestSync is queueBranchExpenseSync's counterpart
// for transfer requests -- same opt-in-if-configured, same-transaction-
// as-the-write behavior.
func queueBranchTransferRequestSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchTransferRequestPayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "transfer_request",
		ClientRef:   payload.ClientRef,
		PayloadJson: string(body),
	})
	return err
}

type branchTransferConfirmItemPayload struct {
	Barcode          string `json:"barcode"`
	ProductUID       string `json:"product_uid"`
	ReceivedQuantity int64  `json:"received_quantity"`
}

// branchTransferConfirmPayload is the exact JSON body POSTed to kashi's
// /branch/transfer_requests/:id/confirm (see api/branch_transfer.go in the
// kashi repo) -- the destination branch's own leg of a transfer, reporting
// what it actually counted. Unlike every other outbox payload, there's no
// natural client_ref (this branch never created the transfer); the outbox
// row's own client_ref is synthesized purely for its own idempotent-retry
// bookkeeping, never sent to kashi or read back.
type branchTransferConfirmPayload struct {
	Items []branchTransferConfirmItemPayload `json:"items"`
	Note  string                             `json:"note"`
}

// queueBranchTransferConfirmSync is queueBranchTransferRequestSync's
// counterpart for the confirm leg.
func queueBranchTransferConfirmSync(ctx context.Context, q *db.Queries, centralAPIURL string, centralID int64, payload branchTransferConfirmPayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "transfer_confirm",
		ClientRef:   fmt.Sprintf("transfer-confirm-%d", centralID),
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
	ClientRef              string `json:"client_ref"`
	Description            string `json:"description"`
	CategoryName           string `json:"category_name"`
	Amount                 int64  `json:"amount"`
	CurrencyCode           string `json:"currency_code"`
	BranchCashboxAccountID int64  `json:"branch_cashbox_account_id"`
	// AccountName -- see branchExpensePayload.AccountName.
	AccountName   string    `json:"account_name"`
	BranchShiftID *int64    `json:"branch_shift_id,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
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

// branchSettlementPayload is the exact JSON body POSTed to kashi's
// /branch/invoice_settlements (see api/branch_invoice_settlement.go in the
// kashi repo — branchSettlementRequest there must stay in sync with this
// shape). A "change settlement" edit (ChangeInvoiceSettlement,
// ChangeInvoiceSettlementAmount, SettleInvoicePayments) rewrites an
// already-synced sale's per-account payment split; kashi replays that onto
// its branch_invoices copy so the admin view of who-paid-with-what never
// goes stale.
//
// SettlementRef is a fresh uuid per edit — each change is its own event,
// deduplicated on retry via (branch_id, client_ref), NOT collapsed with the
// previous settlement of the same sale. SaleClientRef is the sale's stable
// invoice_client_ref (see getOrCreateInvoiceClientRef), the key kashi uses
// to find the branch invoice to update.
type branchSettlementPayload struct {
	SettlementRef string                        `json:"settlement_ref"`
	SaleClientRef string                        `json:"sale_client_ref"`
	GrandTotal    int64                         `json:"grand_total"`
	Payments      []branchInvoicePaymentPayload `json:"payments"`
	OccurredAt    time.Time                     `json:"occurred_at"`
}

// queueBranchSettlementSync is queueBranchInvoiceSync's counterpart for a
// settlement change — same opt-in-if-configured, same-transaction-as-the-
// write behavior. Unlike an invoice's client_ref, SettlementRef is minted
// here when the caller left it blank: every settlement edit is a distinct
// event, so there's no "same edit must reuse the same ref" case to honor.
func queueBranchSettlementSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchSettlementPayload) error {
	if centralAPIURL == "" {
		return nil
	}
	if payload.SettlementRef == "" {
		payload.SettlementRef = uuid.NewString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "invoice_settlement",
		ClientRef:   payload.SettlementRef,
		PayloadJson: string(body),
	})
	return err
}

// queueSettlementChange resolves a sale's stable client_ref and its current
// per-account payment split (as just rewritten by the caller) and queues a
// settlement-change sync for it. It's a no-op when sync isn't configured, so
// all three "change settlement" entry points (ChangeInvoiceSettlement,
// ChangeInvoiceSettlementAmount, SettleInvoicePayments) can call it
// unconditionally from the tail of their write transaction.
func queueSettlementChange(ctx context.Context, q *db.Queries, centralAPIURL string, invoiceID, grandTotal int64) error {
	if centralAPIURL == "" {
		return nil
	}
	saleRef, err := getOrCreateInvoiceClientRef(ctx, q, invoiceID)
	if err != nil {
		return fmt.Errorf("resolve invoice client ref: %w", err)
	}
	payments, err := buildBranchInvoicePayments(ctx, q, invoiceID)
	if err != nil {
		return err
	}
	return queueBranchSettlementSync(ctx, q, centralAPIURL, branchSettlementPayload{
		SaleClientRef: saleRef,
		GrandTotal:    grandTotal,
		Payments:      payments,
		OccurredAt:    time.Now().UTC(),
	})
}

// branchShiftClosePayload is the exact JSON body POSTed to kashi's
// /branch/shift_closes (see api/branch_shift.go in the kashi repo —
// branchShiftCloseRequest there must stay in sync with this shape).
// BranchShiftID is this install's own local shifts.id, meaningless to
// kashi's schema — stored as opaque traceability data, not a foreign key,
// same reasoning as branchInvoicePayload. Expected/counted amounts are the
// exact values printed on the close-shift receipt; kashi derives each
// variance itself (counted − expected) rather than trusting a reported one.
type branchShiftClosePayload struct {
	ShiftRef          string    `json:"shift_ref"`
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
	OccurredAt        time.Time `json:"occurred_at"`
}

// queueBranchShiftCloseSync is queueBranchInvoiceSync's counterpart for a
// shift close. ShiftRef is derived from the local shift id by the caller
// ("shift-<id>"): a shift closes exactly once, so that stable ref is all
// the idempotency kashi needs.
func queueBranchShiftCloseSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchShiftClosePayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "shift_close",
		ClientRef:   payload.ShiftRef,
		PayloadJson: string(body),
	})
	return err
}

// branchVisitorEventPayload is the exact JSON body POSTed to kashi's
// /branch/visitor_events (see api/branch_visitor.go in the kashi repo --
// branchVisitorEventRequest there must stay in sync with this shape). One
// press of the nav-drawer counter = one payload = one outbox row. Ref is
// the visitor_events.client_ref uuid; kashi dedupes on (branch_id, ref).
// Direction is "in" (▲) or "out" (▼). Day is the local "YYYY-MM-DD" the
// press was bucketed under, sent so kashi's per-day rollup matches the
// till's own without re-deriving it from a timezone-shifted occurred_at.
type branchVisitorEventPayload struct {
	Ref        string    `json:"ref"`
	Day        string    `json:"day"`
	Direction  string    `json:"direction"`
	OccurredAt time.Time `json:"occurred_at"`
}

// branchVisitorEventsPayload is the batched body POSTed to kashi's
// /branch/visitor_events_batch. The write path still queues one
// visitor_event outbox row per press (so the local log stays exact); the
// sync worker coalesces whatever visitor_event rows are due in one drain
// into a single payload of this shape, so a burst of counter presses is
// one HTTP round trip instead of one per press. kashi dedupes each element
// on (branch_id, ref), so replaying the whole batch is row-level idempotent.
type branchVisitorEventsPayload struct {
	Events []branchVisitorEventPayload `json:"events"`
}

// queueBranchVisitorEventSync is queueBranchShiftCloseSync's counterpart
// for a single visitor-counter press -- same opt-in-if-configured,
// same-transaction-as-the-write behavior. The caller (AdjustVisitorCountToday)
// only enqueues when the press actually moved the clamped tally, so a
// no-op "out" against a zero count never produces an outbox row.
func queueBranchVisitorEventSync(ctx context.Context, q *db.Queries, centralAPIURL string, payload branchVisitorEventPayload) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}

	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "visitor_event",
		ClientRef:   payload.Ref,
		PayloadJson: string(body),
	})
	return err
}

// branchAttendanceEvent is one row of a branchAttendanceEventsPayload
// batch. Ref is "<device_id>|<event_id>" — already unique per branch, so
// no separate ref column is needed on salesperson_attendance_events.
type branchAttendanceEvent struct {
	Ref              string `json:"ref"`
	SalespersonName  string `json:"salesperson_name"`
	AttendanceUserID string `json:"attendance_user_id"`
	EventDate        string `json:"event_date"`
	EventTime        string `json:"event_time"`
	EventAt          string `json:"event_at"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	// Set on a punch an approved attendance change created: who approved it
	// and what it was, so kashi shows it as changed, not as a device punch.
	ChangedBy  string `json:"changed_by,omitempty"`
	ChangeNote string `json:"change_note,omitempty"`
}

// branchAttendanceEventsPayload is the exact JSON body POSTed to kashi's
// /branch/attendance_events (see api/branch_attendance.go). A single sync
// run's newly-imported punches are batched into one payload — one outbox
// row, one POST — so a first-time backfill of a busy device can't flood the
// outbox. kashi upserts each row on (branch_id, ref), so replaying the
// whole batch is still row-level idempotent.
type branchAttendanceEventsPayload struct {
	Events []branchAttendanceEvent `json:"events"`
}

// branchAttendanceChangePayload is the exact JSON body POSTed to kashi's
// /branch/attendance_changes (see api/branch_attendance.go). It covers both
// a cashier-filed change request ("complaint") and a manager's review of
// one ("approval", carrying the approved/rejected Status and the resulting
// before/after times). Ref is derived from the local
// attendance_change_requests.id by the caller ("acr-<id>-complaint" /
// "acr-<id>-review").
type branchAttendanceChangePayload struct {
	Ref             string    `json:"ref"`
	Kind            string    `json:"kind"`
	SalespersonName string    `json:"salesperson_name"`
	AttendanceDate  string    `json:"attendance_date"`
	OriginalTime    string    `json:"original_time,omitempty"`
	RequestedTime   string    `json:"requested_time,omitempty"`
	RequestedType   string    `json:"requested_type,omitempty"`
	Action          string    `json:"action"`
	Status          string    `json:"status"`
	Note            string    `json:"note,omitempty"`
	Actor           string    `json:"actor,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
	// On an approved replace/delete: the punch it took away here
	// ("<device_id>|<event_id>", the same ref it was uploaded under), so
	// kashi removes its copy too.
	OriginalEventRef string `json:"original_event_ref,omitempty"`
}

// queueSyncOutboxRaw inserts a sync_outbox row on an arbitrary *sql.Tx —
// the escape hatch for code paths that write via raw database/sql rather
// than sqlc's *db.Queries (attendance.go predates the store layer and talks
// to a.conn directly). The atomicity guarantee is the same as
// queueBranchInvoiceSync's: the outbox row commits with the write that
// produced it, or neither does.
func queueSyncOutboxRaw(ctx context.Context, tx *sql.Tx, entityType, clientRef, payloadJSON string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO sync_outbox (entity_type, client_ref, payload_json) VALUES (?, ?, ?)`,
		entityType, clientRef, payloadJSON)
	return err
}
