-- name: GetCashboxAccountByID :one
SELECT * FROM cashbox_accounts WHERE id = ?;

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

-- name: CloseShift :exec
UPDATE shifts SET is_closed = 1, closed_at = ? WHERE id = ?;

-- name: CloseOpenShifts :exec
UPDATE shifts SET is_closed = 1, closed_at = ? WHERE is_closed = 0;

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

-- name: ListShiftAccountBalances :many
SELECT
    t.cashbox_id AS account_id,
    t.shift_id,
    CAST(SUM(t.amount) AS INTEGER) AS balance
FROM (
    SELECT i.shift_id, ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    UNION ALL
    SELECT i.shift_id, i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE NOT EXISTS (
        SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id
    )
) t
WHERE t.shift_id = ?
GROUP BY t.cashbox_id, t.shift_id
ORDER BY t.cashbox_id;

-- name: AddCashboxAccountBalance :one
INSERT INTO shifts_accounts_balances (account_id, shift_id, balance)
VALUES (?, ?, ?)
ON CONFLICT (account_id, shift_id) DO UPDATE SET balance = shifts_accounts_balances.balance + excluded.balance
RETURNING account_id, shift_id, balance;

-- name: GetShiftTotals :many
SELECT s.id, s.is_closed, s.created_at, s.closed_at,
       COALESCE(SUM(i.grand_total), 0) as total
FROM shifts s
LEFT JOIN invoices i ON i.shift_id = s.id
GROUP BY s.id
ORDER BY s.id DESC;
