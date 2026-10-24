-- name: CreateLoan :one
INSERT INTO loans (
  client_ref,
  description,
  category,
  amount,
  currency_code,
  cashbox_account_id,
  shift_id,
  occurred_at,
  created_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now')
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
FROM loans WHERE cancelled_at IS NULL AND datetime(occurred_at, 'localtime') >= ?
GROUP BY currency_code;

-- name: ListLoansForShift :many
SELECT * FROM loans WHERE shift_id = ? AND cancelled_at IS NULL ORDER BY id;

-- name: SetLoanCategory :one
UPDATE loans SET category = ? WHERE id = ? RETURNING *;

-- name: CancelLoanByClientRef :one
-- Stamps cancelled_at once; a repeat matches no row, which the callers treat
-- as already cancelled rather than an error.
UPDATE loans SET cancelled_at = datetime('now') WHERE client_ref = ? AND cancelled_at IS NULL RETURNING *;

-- name: GetLoanByClientRef :one
SELECT * FROM loans WHERE client_ref = ?;
