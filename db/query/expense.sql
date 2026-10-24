-- name: CreateExpense :one
INSERT INTO expenses (
  client_ref,
  description,
  category,
  amount,
  currency_code,
  cashbox_account_id,
  shift_id,
  from_central,
  occurred_at,
  created_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now')
) RETURNING *;

-- name: GetExpense :one
SELECT * FROM expenses WHERE id = ?;

-- name: ListExpenses :many
SELECT * FROM expenses ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: ListAllExpenses :many
SELECT * FROM expenses ORDER BY id;

-- name: SumExpensesByCurrencySince :many
-- Expenses from `since` (a local "YYYY-MM-DD HH:MM:SS"; occurred_at is UTC)
-- on, per currency --
-- the month total under the Expenses button. Amounts are in the expense's
-- own currency, x100, as entered.
SELECT currency_code, CAST(COALESCE(SUM(amount), 0) AS INTEGER) AS total
FROM expenses WHERE cancelled_at IS NULL AND datetime(occurred_at, 'localtime') >= ?
GROUP BY currency_code;

-- name: ListExpensesForShift :many
-- What came out of the shift's drawer: an expense central paid for didn't.
SELECT * FROM expenses WHERE shift_id = ? AND cancelled_at IS NULL AND from_central = 0 ORDER BY id;

-- name: SetExpenseCategory :one
UPDATE expenses SET category = ? WHERE id = ? RETURNING *;

-- name: SetExpenseFromCentral :one
UPDATE expenses SET from_central = ? WHERE client_ref = ? RETURNING *;

-- name: CancelExpenseByClientRef :one
-- Stamps cancelled_at once; a repeat matches no row, which the callers treat
-- as already cancelled rather than an error.
UPDATE expenses SET cancelled_at = datetime('now') WHERE client_ref = ? AND cancelled_at IS NULL RETURNING *;

-- name: GetExpenseByClientRef :one
SELECT * FROM expenses WHERE client_ref = ?;
