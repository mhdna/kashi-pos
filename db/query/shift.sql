-- name: GetCashboxAccountByID :one
SELECT * FROM cashbox_accounts WHERE id = ?;

-- name: GetCashboxAccountByName :one
SELECT * FROM cashbox_accounts WHERE name = ?;

-- name: ListCashboxAccounts :many
SELECT * FROM cashbox_accounts ORDER BY id;

-- name: CreateCashboxAccount :one
INSERT INTO cashbox_accounts (name, currency_code) VALUES (?, ?) RETURNING *;

-- name: DeleteCashboxAccount :exec
DELETE FROM cashbox_accounts WHERE id = ?;

-- name: GetShiftByID :one
SELECT * FROM shifts WHERE id = ?;

-- name: ListShifts :many
SELECT * FROM shifts ORDER BY id;

-- name: GetOpenShift :one
SELECT * FROM shifts WHERE is_closed = 0 LIMIT 1;

-- name: CreateShift :one
INSERT INTO shifts (is_closed, created_at) VALUES (?, datetime('now', 'localtime')) RETURNING *;

-- name: CloseShift :execrows
UPDATE shifts
SET is_closed = 1,
    closed_at = ?,
    closed_by_salesperson_id = ?,
    counted_usd = ?,
    counted_lbp = ?,
    counted_visa = ?,
    counted_whish = ?,
    counted_usd_breakdown = ?,
    expected_usd = ?,
    expected_lbp = ?,
    expected_visa = ?,
    expected_whish = ?
WHERE id = ? AND is_closed = 0;

-- name: CloseOpenShifts :exec
UPDATE shifts SET is_closed = 1, closed_at = ? WHERE is_closed = 0;

-- name: CloseStaleOpenShifts :exec
UPDATE shifts
SET is_closed = 1, closed_at = ?
WHERE is_closed = 0
  AND date(created_at) < date('now', 'localtime');

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = ?;

-- name: ListAccounts :many
SELECT * FROM accounts ORDER BY id;

-- name: CreateAccount :one
INSERT INTO accounts (name) VALUES (?) RETURNING *;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = ?;

-- name: GetShiftAccountBalance :one
SELECT * FROM shifts_accounts_balances WHERE account_id = ? AND shift_id = ?;

-- name: AddCashboxAccountBalance :one
INSERT INTO shifts_accounts_balances (account_id, shift_id, balance)
VALUES (?, ?, ?)
ON CONFLICT (account_id, shift_id) DO UPDATE SET balance = shifts_accounts_balances.balance + excluded.balance
RETURNING account_id, shift_id, balance;

-- name: GetShiftTotals :many
-- Bounded to a single calendar month (the monthly shift table only ever
-- displays one month at a time) instead of loading every shift ever
-- created - pass '' for the historical "everything" behavior.
SELECT s.id, s.is_closed, s.created_at, s.closed_at,
       COALESCE((
           SELECT SUM(i.grand_total)
           FROM invoices i
           WHERE i.shift_id = s.id
       ), 0) as total,
       COALESCE((
           SELECT SUM(MAX(p.price - ip.unit_price, 0) * ip.quantity)
           FROM invoice_products ip
           JOIN products p ON p.id = ip.product_id
           JOIN invoices shift_i ON shift_i.id = ip.invoice_id
           JOIN sales_invoices sales_i ON sales_i.invoice_id = shift_i.id
           WHERE shift_i.shift_id = s.id
       ), 0) AS price_deductions,
       s.counted_usd, s.counted_lbp, s.counted_visa, s.counted_whish,
       s.expected_usd, s.expected_lbp, s.expected_visa, s.expected_whish,
       COALESCE(sp.full_name, '') AS closed_by_salesperson_name
FROM shifts s
LEFT JOIN salespersons sp ON sp.id = s.closed_by_salesperson_id
WHERE (sqlc.arg(totals_month) = '' OR strftime('%Y-%m', s.created_at, 'localtime') = sqlc.arg(totals_month))
ORDER BY s.id DESC;

-- name: GetShiftReportSummary :one
SELECT
    COALESCE((SELECT invoice_code FROM invoices AS first_i WHERE first_i.shift_id = sqlc.arg(report_shift_id) ORDER BY first_i.id LIMIT 1), '') AS first_invoice,
    COALESCE((SELECT invoice_code FROM invoices AS last_i WHERE last_i.shift_id = sqlc.arg(report_shift_id) ORDER BY last_i.id DESC LIMIT 1), '') AS last_invoice,
    (SELECT COUNT(*) FROM invoices AS transaction_i WHERE transaction_i.shift_id = sqlc.arg(report_shift_id)) AS transactions,
    COALESCE((
        SELECT SUM(CASE WHEN ip.quantity > 0 THEN ip.quantity ELSE 0 END)
        FROM invoice_products ip
        JOIN invoices i ON i.id = ip.invoice_id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS items_sold,
    (
        SELECT COUNT(*)
        FROM return_invoices r
        JOIN invoices i ON i.id = r.invoice_id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ) AS returns,
    COALESCE((
        SELECT SUM(i.subtotal)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS gross_sales,
    COALESCE((
        SELECT SUM(i.subtotal - i.discounted_total)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS discounts,
    COALESCE((
        SELECT SUM(MAX(p.price - ip.unit_price, 0) * ip.quantity)
        FROM invoice_products ip
        JOIN products p ON p.id = ip.product_id
        JOIN invoices i ON i.id = ip.invoice_id
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS price_deductions,
    COALESCE((
        SELECT -SUM(CASE WHEN ip.line_total < 0 THEN ip.line_total ELSE 0 END)
        FROM invoice_products ip
        JOIN invoices i ON i.id = ip.invoice_id
        JOIN return_invoices r ON r.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS return_amount,
    COALESCE((
        SELECT SUM(i.grand_total - i.discounted_total)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS tax,
    COALESCE((SELECT SUM(net_i.grand_total) FROM invoices AS net_i WHERE net_i.shift_id = sqlc.arg(report_shift_id)), 0) AS net_sales;

-- name: CreateShiftBalanceReplacement :exec
INSERT INTO shift_balance_replacements (shift_id, from_cashbox_id, to_cashbox_id, amount)
VALUES (?, ?, ?, ?);
