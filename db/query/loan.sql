-- name: CreateLoan :one
INSERT INTO loans (
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

-- name: GetLoan :one
SELECT * FROM loans WHERE id = ?;

-- name: ListLoans :many
SELECT * FROM loans ORDER BY id DESC LIMIT ? OFFSET ?;
