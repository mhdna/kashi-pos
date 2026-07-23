-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = ?;

-- name: ListInvoices :many
SELECT * FROM invoices ORDER BY id;

-- name: ListInvoicesByShiftID :many
SELECT * FROM invoices WHERE shift_id = ? ORDER BY id;

-- name: CreateInvoice :one
INSERT INTO invoices (
    cashbox_id, shift_id, invoice_code, invoice_index,
    year, client_id, inventory_id, discount,
    subtotal, discounted_total, grand_total
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetInvoiceProduct :one
SELECT * FROM invoice_products WHERE invoice_id = ? AND product_id = ?;

-- name: ListInvoiceProducts :many
SELECT * FROM invoice_products WHERE invoice_id = ?;

-- name: CreateInvoiceProduct :exec
INSERT INTO invoice_products (invoice_id, product_id, unit_price, line_total, discount, quantity)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DeleteInvoiceProducts :exec
DELETE FROM invoice_products WHERE invoice_id = ?;

-- name: GetInvoiceIndex :one
SELECT * FROM invoice_indexes WHERE cashbox_id = ? AND year = ? AND type = ?;

-- name: UpsertInvoiceIndex :exec
INSERT INTO invoice_indexes (year, cashbox_id, last_index, type)
VALUES (?, ?, ?, ?)
ON CONFLICT (cashbox_id, year) DO UPDATE SET last_index = excluded.last_index;

-- name: CreateSalesInvoice :exec
INSERT INTO sales_invoices (invoice_id) VALUES (?);

-- name: CreateReturnInvoice :exec
INSERT INTO return_invoices (invoice_id, sales_invoice_id) VALUES (?, ?);

-- name: GetDailyIncomeByMonth :many
SELECT date(created_at) as day, SUM(grand_total) as total
FROM invoices
WHERE strftime('%Y-%m', created_at) = ?
GROUP BY date(created_at)
ORDER BY day;

-- name: GetTodayIncomeByAccount :many
SELECT ca.name, SUM(i.grand_total) as total
FROM invoices i
JOIN cashbox_accounts ca ON ca.id = i.cashbox_id
WHERE date(i.created_at) = date('now')
GROUP BY i.cashbox_id
ORDER BY ca.name;
