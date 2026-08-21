-- name: GetClientByID :one
SELECT * FROM clients WHERE id = ?;

-- name: GetClientByPhone :one
SELECT * FROM clients WHERE phone = ?;

-- name: ListClients :many
SELECT * FROM clients ORDER BY id;

-- name: CountClientsPage :one
SELECT COUNT(*)
FROM clients c
WHERE trim(sqlc.arg(search)) = ''
   OR c.name LIKE '%' || sqlc.arg(search) || '%' COLLATE NOCASE
   OR c.phone LIKE '%' || sqlc.arg(search) || '%';

-- name: CountClients :one
SELECT COUNT(*) FROM clients;

-- name: ListClientsAfter :many
WITH selected_clients AS (
    SELECT c.id, c.name, c.phone, c.total_loyalty_points,
           c.valid_loyalty_points, c.created_at
    FROM clients c
    WHERE c.id > sqlc.arg(after_id)
      AND (
           c.name LIKE '%' || sqlc.arg(search) || '%' COLLATE NOCASE
        OR c.phone LIKE '%' || sqlc.arg(search) || '%'
      )
    ORDER BY c.id
    LIMIT sqlc.arg(limit)
)
SELECT c.id, c.name, c.phone, c.total_loyalty_points,
       c.valid_loyalty_points, c.created_at,
       CAST(COALESCE(SUM(cae.amount - COALESCE((
           SELECT SUM(cap.amount)
           FROM client_account_payments cap
           WHERE cap.invoice_id = cae.invoice_id
       ), 0)), 0) AS INTEGER) AS account_balance
FROM selected_clients c
LEFT JOIN client_account_entries cae ON cae.client_id = c.id
GROUP BY c.id
ORDER BY c.id;

-- name: ListClientsAfterUnfiltered :many
WITH selected_clients AS (
    SELECT c.id, c.name, c.phone, c.total_loyalty_points,
           c.valid_loyalty_points, c.created_at
    FROM clients c
    WHERE c.id > sqlc.arg(after_id)
    ORDER BY c.id
    LIMIT sqlc.arg(limit)
)
SELECT c.id, c.name, c.phone, c.total_loyalty_points,
       c.valid_loyalty_points, c.created_at,
       CAST(COALESCE(SUM(cae.amount - COALESCE((
           SELECT SUM(cap.amount)
           FROM client_account_payments cap
           WHERE cap.invoice_id = cae.invoice_id
       ), 0)), 0) AS INTEGER) AS account_balance
FROM selected_clients c
LEFT JOIN client_account_entries cae ON cae.client_id = c.id
GROUP BY c.id
ORDER BY c.id;

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

-- name: ListCoupons :many
SELECT * FROM coupons ORDER BY created_at DESC;

-- name: CountCouponsByTypeID :one
SELECT COUNT(*) FROM coupons WHERE type_id = ?;

-- name: CountInvoiceCouponPaymentsByTypeID :one
SELECT COUNT(*) FROM invoice_coupon_payments WHERE coupon_type_id = ?;

-- name: CountInvoiceCouponPaymentsByCode :one
SELECT COUNT(*) FROM invoice_coupon_payments WHERE coupon_code = ?;

-- name: DeleteCouponType :exec
DELETE FROM coupon_types WHERE id = ?;

-- name: CreateClientAccountEntry :one
INSERT INTO client_account_entries (client_id, invoice_id, amount)
VALUES (?, ?, ?) RETURNING *;

-- name: GetClientAccountBalance :one
SELECT COALESCE(SUM(cae.amount - COALESCE(p.paid, 0)), 0) AS balance
FROM client_account_entries cae
LEFT JOIN (
    SELECT invoice_id, SUM(amount) AS paid
    FROM client_account_payments
    GROUP BY invoice_id
) p ON p.invoice_id = cae.invoice_id
WHERE cae.client_id = ?;

-- name: GetClientAccountEntryByInvoiceID :one
SELECT * FROM client_account_entries WHERE invoice_id = ?;

-- name: GetClientAccountPaidByInvoiceID :one
SELECT COALESCE(SUM(amount), 0) FROM client_account_payments WHERE invoice_id = ?;

-- name: ListClientAccountEntries :many
SELECT cae.id, cae.client_id, cae.invoice_id, i.invoice_code,
       cae.amount, CAST(COALESCE(p.paid, 0) AS INTEGER) AS paid,
       cae.amount - CAST(COALESCE(p.paid, 0) AS INTEGER) AS outstanding,
       cae.created_at
FROM client_account_entries cae
JOIN invoices i ON i.id = cae.invoice_id
LEFT JOIN (
    SELECT invoice_id, SUM(amount) AS paid
    FROM client_account_payments
    GROUP BY invoice_id
) p ON p.invoice_id = cae.invoice_id
WHERE cae.client_id = ?
ORDER BY cae.created_at DESC, cae.id DESC;

-- name: ListClientAccountPayments :many
SELECT cap.id, cap.invoice_id, cap.cashbox_id, ca.name AS account_name,
       ca.currency_code, cap.shift_id, cap.amount, cap.created_at
FROM client_account_payments cap
JOIN client_account_entries cae ON cae.invoice_id = cap.invoice_id
JOIN cashbox_accounts ca ON ca.id = cap.cashbox_id
WHERE cae.client_id = ?
ORDER BY cap.created_at DESC, cap.id DESC;

-- name: CreateClientAccountPayment :one
INSERT INTO client_account_payments (invoice_id, cashbox_id, shift_id, amount)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: UpdateClientAccountOwnerByInvoice :exec
UPDATE client_account_entries SET client_id = ? WHERE invoice_id = ?;

-- name: UpdateClientAccountEntryAmount :exec
UPDATE client_account_entries SET amount = ? WHERE invoice_id = ?;

-- name: CreateCouponType :one
INSERT INTO coupon_types (name, value_cents, currency_code, is_active, reference_only)
VALUES (?, ?, ?, ?, ?) RETURNING *;

-- name: UpdateCouponType :exec
UPDATE coupon_types
SET name = ?, value_cents = ?, currency_code = ?, is_active = ?, reference_only = ?
WHERE id = ?;

-- name: ListCouponTypes :many
SELECT * FROM coupon_types ORDER BY name;

-- name: GetCouponTypeByID :one
SELECT * FROM coupon_types WHERE id = ?;

-- name: CreateValueCoupon :one
INSERT INTO coupons (code, status, discount_type, reason, client_id, valid_until, type_id, value_cents)
VALUES (?, 'active', 'fixed', ?, ?, ?, ?, ?) RETURNING *;

-- name: GetActiveCouponByCode :one
SELECT * FROM coupons WHERE code = ? COLLATE NOCASE AND status = 'active' AND redeemed_invoice_id IS NULL;

-- name: RedeemCoupon :exec
UPDATE coupons SET status = 'inactive', redeemed_invoice_id = ? WHERE code = ? AND status = 'active' AND redeemed_invoice_id IS NULL;

-- name: CreateInvoiceCouponPayment :one
INSERT INTO invoice_coupon_payments (invoice_id, coupon_code, coupon_type_id, amount)
VALUES (?, ?, ?, ?) RETURNING *;
