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

-- name: ListAllLoans :many
SELECT * FROM loans ORDER BY id;

-- name: SumLoansByCurrencySince :many
-- Same as SumExpensesByCurrencySince, for loans.
SELECT currency_code, CAST(COALESCE(SUM(amount), 0) AS INTEGER) AS total
FROM loans WHERE occurred_at >= ?
GROUP BY currency_code;

-- name: ListLoansForShift :many
SELECT * FROM loans WHERE shift_id = ? ORDER BY id;
