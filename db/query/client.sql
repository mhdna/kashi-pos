-- name: GetClientByID :one
SELECT * FROM clients WHERE id = ?;

-- name: GetClientByPhone :one
SELECT * FROM clients WHERE phone = ?;

-- name: ListClients :many
SELECT * FROM clients ORDER BY id;

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
