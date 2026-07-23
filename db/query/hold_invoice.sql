-- name: CreateHoldInvoice :one
INSERT INTO hold_invoices DEFAULT VALUES RETURNING *;

-- name: AddHoldInvoiceItem :exec
INSERT INTO hold_invoice_items (hold_invoice_id, product_id, qty) VALUES (?, ?, ?);

-- name: ListHoldInvoiceItems :many
SELECT hii.*, p.name, p.code, p.price, p.discount, p.description as category
FROM hold_invoice_items hii
JOIN products p ON p.id = hii.product_id
WHERE hii.hold_invoice_id = ?
ORDER BY hii.id;

-- name: ListHoldInvoices :many
SELECT * FROM hold_invoices ORDER BY id DESC;

-- name: DeleteHoldInvoice :exec
DELETE FROM hold_invoices WHERE id = ?;

-- name: CreateVoidInvoice :one
INSERT INTO void_invoices (items_json, reason) VALUES (?, ?) RETURNING *;
