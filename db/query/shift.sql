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

-- name: CreateShift :one
INSERT INTO shifts (is_closed, created_at, closed_at) VALUES (?, ?, ?) RETURNING *;

-- name: CloseShift :exec
UPDATE shifts SET is_closed = 1, closed_at = ? WHERE id = ?;

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
SELECT * FROM shifts_accounts_balances WHERE shift_id = ?;

-- name: UpsertShiftAccountBalance :exec
INSERT INTO shifts_accounts_balances (account_id, shift_id, balance)
VALUES (?, ?, ?)
ON CONFLICT (account_id, shift_id) DO UPDATE SET balance = excluded.balance;
