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

-- An empty shift (opened and possibly closed, but with no counted money,
-- opening float, or closing salesperson, and -- implied by this check
-- running after CountInvoices/CountExpensesAll/CountLoansAll have all come
-- back zero -- no invoices/expenses/loans against it either) isn't "real"
-- local history: it's what every freshly created branch already has before
-- anyone has actually used it. Only a shift with real activity should block
-- the import.
-- name: CountActiveShiftsAll :one
SELECT COUNT(*) FROM shifts
WHERE closed_by_salesperson_id != 0
   OR counted_usd != 0 OR counted_lbp != 0 OR counted_visa != 0 OR counted_whish != 0
   OR expected_usd != 0 OR expected_lbp != 0 OR expected_visa != 0 OR expected_whish != 0
   OR opening_float_usd != 0 OR opening_float_lbp != 0;

-- PullBranchHistory only reaches these once CountActiveShiftsAll above has
-- already come back zero, i.e. every local shift row left over on this
-- freshly created branch is inert (no counted money, float, invoices,
-- expenses or loans). Clear them out before importing kashi's real shift
-- history so the branch doesn't end up with placeholder rows sitting next
-- to the imported ones.
-- name: DeleteAllShiftReceiptUploads :exec
DELETE FROM shift_receipt_uploads;

-- name: DeleteAllShiftBalanceReplacements :exec
DELETE FROM shift_balance_replacements;

-- name: DeleteAllShiftsAccountsBalances :exec
DELETE FROM shifts_accounts_balances;

-- name: DeleteAllShifts :exec
DELETE FROM shifts;
