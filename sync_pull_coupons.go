package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// Coupons, branches, and transfer requests all pull down from kashi in a
// flatter shape than their central columns actually are -- see
// api/branch_sync_coupons.go in the kashi repo: nullable central columns
// (a coupon's type_id, valid_until, ...) arrive as a plain zero value
// ("", 0) rather than a JSON null or a {Int64, Valid} wrapper, the same
// convention branchInvoiceRequest.ClientUID already uses on the up-sync
// side.

type syncedCouponType struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	ValueCents          int64  `json:"value_cents"`
	CurrencyCode        string `json:"currency_code"`
	IsActive            bool   `json:"is_active"`
	ReferenceOnly       bool   `json:"reference_only"`
	CalculateOnCheckout bool   `json:"calculate_on_checkout"`
	CodeOptional        bool   `json:"code_optional"`
}

// applySyncedCouponType upserts by central_id -- see
// idx_coupon_types_central_id (000101). A type this till has never seen
// (locally created, unsynced) is untouched; one whose central_id already
// matches gets every field overwritten, kashi being authoritative for a
// centrally-managed type from the moment it's synced.
func applySyncedCouponType(ctx context.Context, q *db.Queries, t syncedCouponType) error {
	_, err := q.UpsertCouponTypeByCentralID(ctx, db.UpsertCouponTypeByCentralIDParams{
		CentralID:           sql.NullInt64{Int64: t.ID, Valid: true},
		Name:                t.Name,
		ValueCents:          t.ValueCents,
		CurrencyCode:        t.CurrencyCode,
		IsActive:            boolToInt64(t.IsActive),
		ReferenceOnly:       boolToInt64(t.ReferenceOnly),
		CalculateOnCheckout: boolToInt64(t.CalculateOnCheckout),
		CodeOptional:        boolToInt64(t.CodeOptional),
	})
	return err
}

type syncedCoupon struct {
	Code       string `json:"code"`
	Status     string `json:"status"`
	TypeID     int64  `json:"type_id"`
	ValueCents int64  `json:"value_cents"`
	ValidUntil string `json:"valid_until"`
}

// applySyncedCoupon mirrors one central coupon row. Skipped entirely --
// not merely left with a stale status, but not written to at all -- when
// this till already redeemed the code itself: nothing kashi reports can
// walk that back, and an out-of-order pull landing after the local
// redemption must never resurrect a spent code. type_id is resolved from
// the till's own coupon_types.central_id mapping; a code referencing a
// type this till hasn't pulled yet (the two feeds advance independently)
// is skipped for now and picked up whole once the type arrives, same
// reasoning as applySyncedPriceList skipping an unknown variant.
func applySyncedCoupon(ctx context.Context, q *db.Queries, c syncedCoupon) error {
	existing, err := q.GetCouponByCode(ctx, c.Code)
	if err == nil && existing.RedeemedInvoiceID.Valid {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load existing coupon: %w", err)
	}

	var typeID sql.NullInt64
	if c.TypeID != 0 {
		couponType, err := q.GetCouponTypeByCentralID(ctx, sql.NullInt64{Int64: c.TypeID, Valid: true})
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("load coupon type: %w", err)
		}
		typeID = sql.NullInt64{Int64: couponType.ID, Valid: true}
	}

	var validUntil sql.NullString
	if c.ValidUntil != "" {
		validUntil = sql.NullString{String: c.ValidUntil, Valid: true}
	}

	return q.UpsertSyncedCoupon(ctx, db.UpsertSyncedCouponParams{
		Code:       c.Code,
		Status:     c.Status,
		TypeID:     typeID,
		ValueCents: sql.NullInt64{Int64: c.ValueCents, Valid: true},
		ValidUntil: validUntil,
	})
}

type syncedBranch struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	IsActive bool   `json:"is_active"`
}

func applySyncedBranch(ctx context.Context, q *db.Queries, b syncedBranch) error {
	return q.UpsertSyncedBranch(ctx, db.UpsertSyncedBranchParams{
		Code:     b.Code,
		Name:     b.Name,
		IsActive: boolToInt64(b.IsActive),
	})
}

type syncedTransferRequest struct {
	ID        int64  `json:"id"`
	ClientRef string `json:"client_ref"`
	Status    string `json:"status"`
}

// applySyncedTransferRequest is the till's own poll for what happened to
// a transfer it proposed -- see notifyBranchTransferDecision in the kashi
// repo. kashi's status is authoritative the moment it's synced; a
// client_ref this till doesn't recognise (a transfer created centrally
// with no matching local row, or one from another till on a shared
// database -- neither should happen in practice) is a silent no-op, not
// an error worth failing the whole pull over.
func applySyncedTransferRequest(ctx context.Context, q *db.Queries, t syncedTransferRequest) error {
	if t.ClientRef == "" {
		return nil
	}
	return q.SetTransferRequestStatus(ctx, db.SetTransferRequestStatusParams{
		Status:    t.Status,
		ClientRef: t.ClientRef,
	})
}

type syncedIncomingTransferItem struct {
	Barcode          string `json:"barcode"`
	ProductUID       string `json:"product_uid"`
	Quantity         int64  `json:"quantity"`
	ReceivedQuantity int64  `json:"received_quantity"`
}

type syncedIncomingTransfer struct {
	ID             int64                        `json:"id"`
	FromBranchName string                       `json:"from_branch_name"`
	Note           string                       `json:"note"`
	Status         string                       `json:"status"`
	Items          []syncedIncomingTransferItem `json:"items"`
}

// applySyncedIncomingTransfer mirrors one transfer addressed to this
// branch, whole -- same replace-in-transaction shape as
// applySyncedPriceList: the row is upserted by central_id, its items are
// dropped and reinserted from the payload. This branch's own counted
// received_quantity (set locally via ConfirmTransferRequest, not synced
// down) would be wiped by that replace, so it's re-applied from the local
// row's own items afterward rather than trusting the central payload's
// received_quantity for lines this till has already counted itself.
func applySyncedIncomingTransfer(ctx context.Context, q *db.Queries, t syncedIncomingTransfer) error {
	existing, err := q.GetTransferRequestByCentralID(ctx, sql.NullInt64{Int64: t.ID, Valid: true})
	hadExisting := err == nil
	var priorCounts map[string]int64
	if hadExisting {
		priorItems, err := q.ListTransferRequestItems(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("load existing items: %w", err)
		}
		priorCounts = make(map[string]int64, len(priorItems))
		for _, it := range priorItems {
			if it.ReceivedQuantity.Valid {
				priorCounts[it.Barcode] = it.ReceivedQuantity.Int64
			}
		}
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("load existing transfer request: %w", err)
	}

	transfer, err := q.UpsertIncomingTransferRequest(ctx, db.UpsertIncomingTransferRequestParams{
		CentralID:        sql.NullInt64{Int64: t.ID, Valid: true},
		Note:             t.Note,
		Status:           t.Status,
		SenderBranchName: t.FromBranchName,
	})
	if err != nil {
		return fmt.Errorf("upsert transfer request: %w", err)
	}

	if err := q.DeleteTransferRequestItems(ctx, transfer.ID); err != nil {
		return fmt.Errorf("clear items: %w", err)
	}
	for _, item := range t.Items {
		if item.Barcode == "" {
			continue
		}
		created, err := q.CreateTransferRequestItem(ctx, db.CreateTransferRequestItemParams{
			TransferRequestID: transfer.ID,
			Barcode:           item.Barcode,
			ProductUid:        item.ProductUID,
			Quantity:          item.Quantity,
		})
		if err != nil {
			return fmt.Errorf("create item: %w", err)
		}
		if count, ok := priorCounts[item.Barcode]; ok {
			if err := q.SetTransferRequestItemReceivedQuantity(ctx, db.SetTransferRequestItemReceivedQuantityParams{
				ID:               created.ID,
				ReceivedQuantity: sql.NullInt64{Int64: count, Valid: true},
			}); err != nil {
				return fmt.Errorf("restore counted quantity: %w", err)
			}
		}
	}
	return nil
}

type syncedBranchStock struct {
	BranchCode string `json:"branch_code"`
	VariantUID string `json:"variant_uid"`
	Barcode    string `json:"barcode"`
	Quantity   int64  `json:"quantity"`
	Price      int64  `json:"price"`
}

func applySyncedBranchStock(ctx context.Context, q *db.Queries, s syncedBranchStock) error {
	return q.UpsertBranchStock(ctx, db.UpsertBranchStockParams{
		BranchCode: s.BranchCode,
		VariantUid: s.VariantUID,
		Barcode:    s.Barcode,
		Quantity:   s.Quantity,
		PriceCents: s.Price,
		UpdatedAt:  time.Now().UTC().Format("2006-01-02 15:04:05"),
	})
}
