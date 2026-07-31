-- name: GetClientByID :one
SELECT * FROM clients WHERE id = ?;

-- name: GetClientByPhone :one
SELECT * FROM clients WHERE phone = ?;

-- name: ListClients :many
SELECT * FROM clients ORDER BY id;

-- name: SearchClients :many
SELECT *
FROM clients
WHERE trim(sqlc.arg(search)) = ''
   OR name LIKE '%' || sqlc.arg(search) || '%' COLLATE NOCASE
   OR phone LIKE '%' || sqlc.arg(search) || '%'
ORDER BY id
LIMIT sqlc.arg(result_limit);

-- name: CreateClient :one
INSERT INTO clients (name, phone, total_loyalty_points, valid_loyalty_points)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: UpdateClient :one
UPDATE clients
SET name = ?,
    phone = ?,
    total_loyalty_points = ?,
    valid_loyalty_points = ?
WHERE id = ? RETURNING *;

-- name: AddClientLoyaltyPoints :exec
UPDATE clients
SET total_loyalty_points = total_loyalty_points + ?,
    valid_loyalty_points = valid_loyalty_points + ?
WHERE id = ?;

-- name: DeleteClient :exec
DELETE FROM clients WHERE id = ?;

-- name: GetCouponByCode :one
SELECT * FROM coupons WHERE code = ?;

-- name: ListCouponsByClientID :many
SELECT * FROM coupons WHERE client_id = ? ORDER BY created_at;

-- name: CreateCoupon :one
INSERT INTO coupons (code, status, discount_type, reason, client_id, valid_until)
VALUES (?, ?, ?, ?, ?, ?) RETURNING *;

-- name: UpdateCouponStatus :exec
UPDATE coupons SET status = ? WHERE code = ?;

-- name: DeleteCoupon :exec
DELETE FROM coupons WHERE code = ?;

-- name: CreateClientAccountEntry :one
INSERT INTO client_account_entries (client_id, invoice_id, amount)
VALUES (?, ?, ?) RETURNING *;

-- name: GetClientAccountBalance :one
SELECT COALESCE(SUM(amount), 0) AS balance
FROM client_account_entries WHERE client_id = ?;

-- name: CreateCouponType :one
INSERT INTO coupon_types (name, value_cents, currency_code, is_active)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: ListCouponTypes :many
SELECT * FROM coupon_types ORDER BY name;

-- name: GetCouponTypeByID :one
SELECT * FROM coupon_types WHERE id = ?;

-- name: CreateValueCoupon :one
INSERT INTO coupons (code, status, discount_type, reason, client_id, valid_until, type_id, value_cents)
VALUES (?, 'active', 'fixed', ?, ?, ?, ?, ?) RETURNING *;

-- name: GetActiveCouponByCode :one
SELECT * FROM coupons WHERE code = ? AND status = 'active' AND redeemed_invoice_id IS NULL;

-- name: RedeemCoupon :exec
UPDATE coupons SET status = 'inactive', redeemed_invoice_id = ? WHERE code = ? AND status = 'active' AND redeemed_invoice_id IS NULL;

-- name: CreateInvoiceCouponPayment :one
INSERT INTO invoice_coupon_payments (invoice_id, coupon_code, coupon_type_id, amount)
VALUES (?, ?, ?, ?) RETURNING *;
