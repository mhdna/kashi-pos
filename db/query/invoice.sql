-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = ?;

-- name: ListInvoicePayments :many
SELECT * FROM invoice_payments WHERE invoice_id = ? ORDER BY cashbox_id;

-- name: DeleteInvoicePayments :exec
DELETE FROM invoice_payments WHERE invoice_id = ?;

-- name: DeleteInvoicePaymentEntries :exec
DELETE FROM invoice_payment_entries WHERE invoice_id = ?;

-- name: UpdateInvoiceClient :exec
UPDATE invoices SET client_id = ? WHERE id = ?;

-- name: UpdateInvoiceSalesperson :exec
UPDATE invoices SET salesperson_id = ? WHERE id = ?;

-- name: UpdateInvoiceCashbox :exec
UPDATE invoices SET cashbox_id = ? WHERE id = ?;

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
SELECT COALESCE(SUM(CASE WHEN ip.quantity < 0 THEN -ip.quantity ELSE 0 END), 0) AS quantity
FROM return_invoices r
JOIN invoice_products ip ON ip.invoice_id = r.invoice_id
WHERE r.sales_invoice_id = ? AND ip.product_id = ?;

-- name: GetReturnedQuantityForProductPrice :one
SELECT COALESCE(SUM(CASE WHEN ip.quantity < 0 THEN -ip.quantity ELSE 0 END), 0) AS quantity
FROM return_invoices r
JOIN invoice_products ip ON ip.invoice_id = r.invoice_id
WHERE r.sales_invoice_id = ? AND ip.product_id = ? AND ip.unit_price = ? AND ip.discount = ?;

-- name: ListInvoices :many
SELECT * FROM invoices ORDER BY id;

-- name: ListInvoicesPage :many
SELECT i.*, COALESCE(s.full_name, 'Anonymous') AS salesperson_name
FROM invoices i
LEFT JOIN salespersons s ON s.id = i.salesperson_id
ORDER BY i.id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountInvoices :one
SELECT COUNT(*) FROM invoices;

-- name: ListInvoicesByShiftID :many
SELECT * FROM invoices WHERE shift_id = ? ORDER BY id;

-- name: CreateInvoice :one
INSERT INTO invoices (
    cashbox_id, shift_id, invoice_code, invoice_index,
    year, client_id, inventory_id, discount,
    subtotal, discounted_total, grand_total, salesperson_id, notes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

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

-- name: DeleteSalesInvoice :exec
DELETE FROM sales_invoices WHERE invoice_id = ?;

-- name: DeleteInvoice :exec
DELETE FROM invoices WHERE id = ?;

-- name: GetInvoiceIndex :one
SELECT * FROM invoice_indexes WHERE year = ? AND type = ?;

-- name: UpsertInvoiceIndex :exec
INSERT INTO invoice_indexes (year, last_index, type)
VALUES (?, ?, ?)
ON CONFLICT (year, type) DO UPDATE SET last_index = excluded.last_index;

-- name: CreateSalesInvoice :exec
INSERT INTO sales_invoices (invoice_id) VALUES (?);

-- name: CreateReturnInvoice :exec
INSERT INTO return_invoices (invoice_id, sales_invoice_id) VALUES (?, ?);

-- name: GetDailyIncomeByMonth :many
WITH collected AS (
    SELECT date(i.created_at, 'localtime') AS income_day, ip.amount AS collected_amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    JOIN cashbox_accounts ca ON ca.id = ip.cashbox_id
    WHERE lower(trim(ca.name)) <> 'on account'
      AND strftime('%Y-%m', i.created_at, 'localtime') = sqlc.arg(year_month)
    UNION ALL
    SELECT date(i.created_at, 'localtime') AS income_day, i.grand_total AS collected_amount
    FROM invoices i
    JOIN cashbox_accounts ca ON ca.id = i.cashbox_id
    WHERE NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
      AND lower(trim(ca.name)) <> 'on account'
      AND strftime('%Y-%m', i.created_at, 'localtime') = sqlc.arg(year_month)
    UNION ALL
    SELECT date(cap.created_at, 'localtime') AS income_day, cap.amount AS collected_amount
    FROM client_account_payments cap
    WHERE strftime('%Y-%m', cap.created_at, 'localtime') = sqlc.arg(year_month)
)
SELECT income_day as day, SUM(collected_amount) as total
FROM collected
GROUP BY income_day
ORDER BY day;

-- name: IncrementInvoicesIndex :one
INSERT INTO invoice_indexes (year, last_index, type)
VALUES (?, 1, ?)
ON CONFLICT (year, type) DO UPDATE SET last_index = last_index + 1
RETURNING last_index;

-- name: GetTodayIncomeByAccount :many
WITH invoice_account_totals AS (
    SELECT i.created_at, ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    UNION ALL
    SELECT i.created_at, i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE NOT EXISTS (
        SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id
    )
    UNION ALL
    SELECT cap.created_at, cap.cashbox_id, cap.amount
    FROM client_account_payments cap
)
SELECT ca.name, ca.currency_code, SUM(t.amount) as total
FROM invoice_account_totals t
JOIN cashbox_accounts ca ON ca.id = t.cashbox_id
WHERE date(t.created_at, 'localtime') = date('now', 'localtime')
GROUP BY t.cashbox_id
ORDER BY ca.name;
