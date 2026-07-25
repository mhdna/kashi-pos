-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = ?;

-- name: GetSalesInvoiceByCode :one
SELECT i.*
FROM invoices i
JOIN sales_invoices s ON s.invoice_id = i.id
WHERE lower(i.invoice_code) = lower(?);

-- name: GetSalesInvoiceByID :one
SELECT i.*
FROM invoices i
JOIN sales_invoices s ON s.invoice_id = i.id
WHERE i.id = ?;

-- name: GetReturnedQuantityForProduct :one
SELECT COALESCE(SUM(ip.quantity), 0) AS quantity
FROM return_invoices r
JOIN invoice_products ip ON ip.invoice_id = r.invoice_id
WHERE r.sales_invoice_id = ? AND ip.product_id = ?;

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

-- name: CreateInvoicePayment :exec
INSERT INTO invoice_payments (invoice_id, cashbox_id, amount) VALUES (?, ?, ?)
ON CONFLICT (invoice_id, cashbox_id) DO UPDATE SET amount = invoice_payments.amount + excluded.amount;

-- name: CreateInvoicePaymentEntry :exec
INSERT INTO invoice_payment_entries (invoice_id, cashbox_id, amount, entry_type)
VALUES (?, ?, ?, ?);

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

-- name: IncrementInvoicesIndex :one
INSERT INTO invoice_indexes (year, cashbox_id, last_index, type)
VALUES (?, ?, 1, ?)
ON CONFLICT (cashbox_id, year, type) DO UPDATE SET last_index = last_index + 1
RETURNING last_index;

-- name: GetTodayIncomeByAccount :many
SELECT ca.name, SUM(i.grand_total) as total
FROM invoices i
JOIN cashbox_accounts ca ON ca.id = i.cashbox_id
WHERE date(i.created_at) = date('now')
GROUP BY i.cashbox_id
ORDER BY ca.name;
