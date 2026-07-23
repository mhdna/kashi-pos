-- name: CreateHoldInvoice :one
INSERT INTO hold_invoices (items_json) VALUES (?) RETURNING *;

-- name: ListHoldInvoices :many
SELECT * FROM hold_invoices ORDER BY id;

-- name: DeleteHoldInvoice :exec
DELETE FROM hold_invoices WHERE id = ?;

-- name: CreateVoidInvoice :one
INSERT INTO void_invoices (items_json, reason) VALUES (?, ?) RETURNING *;
