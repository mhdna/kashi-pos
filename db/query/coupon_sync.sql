-- name: GetCouponTypeByCentralID :one
SELECT * FROM coupon_types WHERE central_id = ?;

-- name: UpsertCouponTypeByCentralID :one
INSERT INTO coupon_types (central_id, name, value_cents, currency_code, is_active, reference_only, calculate_on_checkout, code_optional)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(central_id) DO UPDATE SET
    name = excluded.name,
    value_cents = excluded.value_cents,
    currency_code = excluded.currency_code,
    is_active = excluded.is_active,
    reference_only = excluded.reference_only,
    calculate_on_checkout = excluded.calculate_on_checkout,
    code_optional = excluded.code_optional
RETURNING *;

-- name: UpsertSyncedCoupon :exec
-- The caller (applySyncedCoupon) checks redeemed_invoice_id first and
-- skips calling this at all for a coupon this till already redeemed
-- itself -- nothing kashi reports should walk that back. discount_type/
-- reason are left NULL on a fresh insert, same as any other
-- centrally-authored coupon (see the coupons table's own comment).
INSERT INTO coupons (code, status, type_id, value_cents, valid_until)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(code) DO UPDATE SET
    status = excluded.status,
    type_id = excluded.type_id,
    value_cents = excluded.value_cents,
    valid_until = excluded.valid_until;

-- name: UpsertSyncedBranch :exec
INSERT INTO synced_branches (code, name, is_active)
VALUES (?, ?, ?)
ON CONFLICT(code) DO UPDATE SET name = excluded.name, is_active = excluded.is_active;

-- name: ListSyncedBranches :many
SELECT * FROM synced_branches WHERE is_active = 1 ORDER BY name;

-- name: CreateTransferRequest :one
INSERT INTO transfer_requests (client_ref, to_branch_code, note)
VALUES (?, ?, ?) RETURNING *;

-- name: CreateTransferRequestItem :one
INSERT INTO transfer_request_items (transfer_request_id, barcode, product_uid, quantity)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: GetTransferRequestByClientRef :one
SELECT * FROM transfer_requests WHERE client_ref = ?;

-- name: ListTransferRequestItems :many
SELECT * FROM transfer_request_items WHERE transfer_request_id = ?;

-- name: ListTransferRequests :many
SELECT * FROM transfer_requests ORDER BY created_at DESC;

-- name: SetTransferRequestStatus :exec
-- Applied from the transfer_requests down-sync: kashi's status is
-- authoritative the moment it's synced (see approveTransfer/
-- notifyBranchTransferDecision in the kashi repo).
UPDATE transfer_requests SET status = ?, updated_at = datetime('now') WHERE client_ref = ?;
