-- ---------------------------------------------------------------------------
-- Historical-import variants of the ordinary write path (CreateShift,
-- CreateInvoice, CreateExpense, CreateLoan). Those hardcode/default their
-- timestamp to "now" because a live write always happens now; pulling a
-- record back down from kashi needs to set it to when the thing actually
-- happened, which may be years in the past. See sync_branch_history.go.
-- Deliberately separate queries rather than adding an optional timestamp
-- param to the live ones, so the live checkout/expense/loan/shift-close
-- paths can't accidentally be called with the wrong timestamp.
-- ---------------------------------------------------------------------------

-- name: CreateHistoricalShift :one
INSERT INTO shifts (
  is_closed, created_at, closed_at, closed_by_salesperson_id,
  counted_usd, counted_lbp, counted_usd_breakdown, counted_visa, counted_whish,
  expected_usd, expected_lbp, expected_visa, expected_whish,
  opening_float_usd, opening_float_lbp, client_ref
) VALUES (
  1, ?, ?, 0,
  ?, ?, '{}', ?, ?,
  ?, ?, ?, ?,
  ?, ?, ?
) RETURNING *;

-- name: CreateHistoricalInvoice :one
INSERT INTO invoices (
    cashbox_id, shift_id, invoice_code, invoice_index,
    year, client_id, inventory_id, discount,
    subtotal, discounted_total, grand_total, salesperson_id, notes, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: CreateHistoricalExpense :one
INSERT INTO expenses (
  client_ref, description, category, amount, currency_code,
  cashbox_account_id, shift_id, occurred_at, created_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: CreateHistoricalLoan :one
INSERT INTO loans (
  client_ref, description, category, amount, currency_code,
  cashbox_account_id, shift_id, occurred_at, created_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetShiftByClientRef :one
SELECT * FROM shifts WHERE client_ref = ? LIMIT 1;

-- name: CountExpensesAll :one
SELECT COUNT(*) FROM expenses;

-- name: CountLoansAll :one
SELECT COUNT(*) FROM loans;

-- name: CountShiftsAll :one
SELECT COUNT(*) FROM shifts;
