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

-- name: ListOutgoingTransferRequests :many
SELECT * FROM transfer_requests WHERE direction = 'outgoing' ORDER BY created_at DESC;

-- name: ListIncomingTransferRequests :many
SELECT * FROM transfer_requests WHERE direction = 'incoming' ORDER BY created_at DESC;

-- name: SetTransferRequestStatus :exec
-- Applied from the transfer_requests down-sync: kashi's status is
-- authoritative the moment it's synced (see approveTransfer/
-- notifyBranchTransferDecision in the kashi repo).
UPDATE transfer_requests SET status = ?, updated_at = datetime('now') WHERE client_ref = ?;

-- name: GetTransferRequestByCentralID :one
SELECT * FROM transfer_requests WHERE central_id = ?;

-- name: UpsertIncomingTransferRequest :one
-- Incoming transfers sync whole, like a price list -- see
-- applySyncedIncomingTransfer. client_ref has no meaning for a transfer
-- this till never created; a synthetic placeholder just satisfies the
-- column's NOT NULL UNIQUE (it's never read back by ref for these rows,
-- only by central_id).
INSERT INTO transfer_requests (client_ref, to_branch_code, note, status, direction, central_id, sender_branch_name)
VALUES ('incoming-' || ?, '', ?, ?, 'incoming', ?, ?)
ON CONFLICT(central_id) DO UPDATE SET
    note = excluded.note,
    status = excluded.status,
    sender_branch_name = excluded.sender_branch_name,
    updated_at = datetime('now')
RETURNING *;

-- name: DeleteTransferRequestItems :exec
DELETE FROM transfer_request_items WHERE transfer_request_id = ?;

-- name: SetTransferRequestItemReceivedQuantity :exec
UPDATE transfer_request_items SET received_quantity = ? WHERE id = ?;

-- ---------------------------------------------------------------------------
-- Cross-branch stock visibility -- a pure mirror, upserted whole by
-- (branch_code, variant_uid) from kashi's branch_stock feed. Search joins
-- against the local products table (matched by uid) for a display name;
-- a stock row for a variant this till hasn't itself pulled down yet still
-- shows (barcode only) rather than being hidden.
-- ---------------------------------------------------------------------------

-- name: UpsertBranchStock :exec
INSERT INTO branch_stock (branch_code, variant_uid, barcode, quantity, price_cents, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(branch_code, variant_uid) DO UPDATE SET
    barcode = excluded.barcode,
    quantity = excluded.quantity,
    price_cents = excluded.price_cents,
    updated_at = excluded.updated_at;

-- name: SearchBranchStock :many
SELECT
    bs.branch_code, bs.variant_uid, bs.barcode, bs.quantity, bs.price_cents,
    COALESCE(p.name, '') AS product_name,
    COALESCE(p.code, '') AS product_code
FROM branch_stock bs
LEFT JOIN products p ON p.uid = bs.variant_uid
WHERE trim(sqlc.arg(search)) = ''
   OR bs.barcode LIKE '%' || sqlc.arg(search) || '%'
   OR p.name LIKE '%' || sqlc.arg(search) || '%' COLLATE NOCASE
   OR p.code LIKE '%' || sqlc.arg(search) || '%' COLLATE NOCASE
ORDER BY p.name, bs.barcode
LIMIT 200;
