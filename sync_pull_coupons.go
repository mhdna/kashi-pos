package main

import (
	"context"
	"database/sql"
	"fmt"

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
