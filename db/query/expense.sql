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
