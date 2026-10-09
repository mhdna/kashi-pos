-- name: CreateExpense :one
INSERT INTO expenses (
  client_ref,
  description,
  category,
  amount,
  currency_code,
  cashbox_account_id,
  shift_id
) VALUES (
  ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetExpense :one
SELECT * FROM expenses WHERE id = ?;

-- name: ListExpenses :many
SELECT * FROM expenses ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: ListAllExpenses :many
SELECT * FROM expenses ORDER BY id;

-- name: SumExpensesByCurrencySince :many
-- Expenses from `since` (a local "YYYY-MM-DD HH:MM:SS") on, per currency --
-- the month total under the Expenses button. Amounts are in the expense's
-- own currency, x100, as entered.
SELECT currency_code, CAST(COALESCE(SUM(amount), 0) AS INTEGER) AS total
FROM expenses WHERE occurred_at >= ?
GROUP BY currency_code;

-- name: ListExpensesForShift :many
SELECT * FROM expenses WHERE shift_id = ? ORDER BY id;
