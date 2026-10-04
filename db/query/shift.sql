-- name: GetCashboxAccountByID :one
SELECT * FROM cashbox_accounts WHERE id = ?;

-- name: GetCashboxAccountByName :one
SELECT * FROM cashbox_accounts WHERE name = ?;

-- name: ListCashboxAccounts :many
-- Every account, active or not. Used where history has to resolve (shift
-- reports, invoice detail) -- a retired method still has to have a name.
SELECT * FROM cashbox_accounts ORDER BY sort_order, id;

-- name: ListActiveCashboxAccounts :many
-- What a cashier may actually take payment with. A method kashi has retired
-- is deactivated rather than deleted, so this is the list the checkout
-- calculator and the settings screen read.
SELECT * FROM cashbox_accounts WHERE is_active = 1 ORDER BY sort_order, id;

-- name: SetCashboxAccountActive :exec
UPDATE cashbox_accounts SET is_active = ? WHERE id = ?;

-- name: DeactivateCashboxAccountsNotIn :exec
-- The reconcile half of the payment-method sync: anything kashi did not send
-- stops being offered. The two internal placeholders are exempt -- kashi has
-- no concept of them and the coupon/account settlement paths look them up by
-- exact name.
UPDATE cashbox_accounts
SET is_active = 0
WHERE name NOT IN (sqlc.slice('names'))
  AND name NOT IN ('On Account', 'On Coupon');

-- name: CreateCashboxAccount :one
-- New accounts append to the end of the admin-defined order automatically.
INSERT INTO cashbox_accounts (name, currency_code, sort_order)
VALUES (?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM cashbox_accounts))
RETURNING *;

-- name: UpdateCashboxAccount :one
UPDATE cashbox_accounts SET name = ?, currency_code = ?, color = ? WHERE id = ? RETURNING *;

-- name: DeleteCashboxAccount :exec
DELETE FROM cashbox_accounts WHERE id = ?;

-- name: SetCashboxAccountSortOrder :exec
UPDATE cashbox_accounts SET sort_order = ? WHERE id = ?;

-- name: GetShiftByID :one
SELECT * FROM shifts WHERE id = ?;

-- name: ListShifts :many
SELECT * FROM shifts ORDER BY id;

-- name: GetOpenShift :one
SELECT * FROM shifts WHERE is_closed = 0 LIMIT 1;

-- name: CreateShift :one
INSERT INTO shifts (is_closed, created_at, opening_float_usd, opening_float_lbp)
VALUES (?, datetime('now'), ?, ?) RETURNING *;

-- name: UpdateShiftOpeningFloat :one
UPDATE shifts SET opening_float_usd = ?, opening_float_lbp = ? WHERE id = ? AND is_closed = 0 RETURNING *;

-- name: CloseShift :execrows
UPDATE shifts
SET is_closed = 1,
    closed_at = ?,
    closed_by_salesperson_id = ?,
    counted_usd = ?,
    counted_lbp = ?,
    counted_visa = ?,
    counted_whish = ?,
    counted_usd_breakdown = ?,
    expected_usd = ?,
    expected_lbp = ?,
    expected_visa = ?,
    expected_whish = ?
WHERE id = ? AND is_closed = 0;

-- name: CloseOpenShifts :exec
UPDATE shifts SET is_closed = 1, closed_at = ? WHERE is_closed = 0;

-- name: DeleteEmptyStaleOpenShifts :execrows
-- A stale shift nothing ever happened in ($0, no rows of any kind point at
-- it) is dropped instead of auto-closed, so it never shows up in the shifts
-- list or gets pushed to kashi as a shift_close. Runs right before
-- CloseStaleOpenShifts, with the same staleness test. Every table with a
-- shift_id column must be listed here.
DELETE FROM shifts
WHERE is_closed = 0
  AND date(created_at, 'localtime') < date('now', 'localtime')
  AND NOT EXISTS (SELECT 1 FROM invoices WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM expenses WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM loans WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM client_account_payments WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM shift_balance_replacements WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM shifts_accounts_balances WHERE shift_id = shifts.id)
  AND NOT EXISTS (SELECT 1 FROM shift_receipt_uploads WHERE shift_id = shifts.id);

-- name: CloseStaleOpenShifts :exec
UPDATE shifts
SET is_closed = 1, closed_at = ?
WHERE is_closed = 0
  AND date(created_at, 'localtime') < date('now', 'localtime');

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

-- name: AddCashboxAccountBalance :one
INSERT INTO shifts_accounts_balances (account_id, shift_id, balance)
VALUES (?, ?, ?)
ON CONFLICT (account_id, shift_id) DO UPDATE SET balance = shifts_accounts_balances.balance + excluded.balance
RETURNING account_id, shift_id, balance;

-- name: GetShiftTotals :many
-- Bounded to a single calendar month (the monthly shift table only ever
-- displays one month at a time) instead of loading every shift ever
-- created - pass '' for the historical "everything" behavior.
SELECT s.id, s.is_closed, s.created_at, s.closed_at,
       COALESCE((
           SELECT SUM(i.grand_total)
           FROM invoices i
           WHERE i.shift_id = s.id
       ), 0) - COALESCE((
           -- Same deferred-coupon exclusion as GetShiftReportSummary's
           -- net_sales: a "calculate on checkout" coupon is settled later
           -- by whoever issued it, not collected by the branch now, so it
           -- must not inflate this shift's displayed total either.
           SELECT SUM(icp.amount)
           FROM invoice_coupon_payments icp
           JOIN coupon_types ct ON ct.id = icp.coupon_type_id
           JOIN invoices i ON i.id = icp.invoice_id
           WHERE i.shift_id = s.id AND ct.calculate_on_checkout = 1
       ), 0) as total,
       COALESCE((
           SELECT SUM(MAX(p.price - ip.unit_price, 0) * ip.quantity)
           FROM invoice_products ip
           JOIN products p ON p.id = ip.product_id
           JOIN invoices shift_i ON shift_i.id = ip.invoice_id
           JOIN sales_invoices sales_i ON sales_i.invoice_id = shift_i.id
           WHERE shift_i.shift_id = s.id
       ), 0) AS price_deductions,
       s.counted_usd, s.counted_lbp, s.counted_visa, s.counted_whish,
       s.expected_usd, s.expected_lbp, s.expected_visa, s.expected_whish,
       COALESCE(sp.full_name, '') AS closed_by_salesperson_name
FROM shifts s
LEFT JOIN salespersons sp ON sp.id = s.closed_by_salesperson_id
WHERE (sqlc.arg(totals_month) = '' OR strftime('%Y-%m', s.created_at, 'localtime') = sqlc.arg(totals_month))
ORDER BY s.id DESC;

-- name: GetDeferredCouponTotalByMonth :one
-- The total "calculate on checkout" coupon amount excluded from this same
-- month's GetShiftTotals figures, surfaced separately so the balance page
-- can tell the cashier why the total is lower than a raw sum of invoices
-- instead of the number just changing silently. Same month-scoping
-- convention as GetShiftTotals (pass '' for all time).
SELECT COALESCE(SUM(icp.amount), 0)
FROM invoice_coupon_payments icp
JOIN coupon_types ct ON ct.id = icp.coupon_type_id
JOIN invoices i ON i.id = icp.invoice_id
JOIN shifts s ON s.id = i.shift_id
WHERE (sqlc.arg(totals_month) = '' OR strftime('%Y-%m', s.created_at, 'localtime') = sqlc.arg(totals_month))
  AND ct.calculate_on_checkout = 1;

-- name: GetShiftReportSummary :one
-- shift_tax recomputes the tax charged per invoice exactly the way checkout
-- does (priceInvoiceTotals -> roundFloatToCents): round(discounted_total *
-- tax_rate) under the configured rounding mode. Invoices don't store their
-- own tax, so a shift printed after the rate changes is priced at the
-- current rate; cash_rounding below absorbs any difference, so the report
-- balances either way.
WITH shift_tax AS (
    -- The /100.0 ... *100.0 detour looks redundant but is not: it reproduces
    -- roundFloatToCents' float64 arithmetic bit for bit, so exact .5 ties
    -- land on the same side here as they did at checkout.
    SELECT SUM(
        CASE cfg.rounding_mode
            WHEN 'down' THEN CAST(i.discounted_total / 100.0 * cfg.tax_rate * 100.0 + 0.000000001 AS INTEGER)
            WHEN 'up' THEN CAST(i.discounted_total / 100.0 * cfg.tax_rate * 100.0 - 0.000000001 AS INTEGER)
                + (i.discounted_total / 100.0 * cfg.tax_rate * 100.0 - 0.000000001
                    > CAST(i.discounted_total / 100.0 * cfg.tax_rate * 100.0 - 0.000000001 AS INTEGER))
            ELSE CAST(i.discounted_total / 100.0 * cfg.tax_rate * 100.0 + 0.5 AS INTEGER)
        END
    ) AS tax
    FROM invoices i
    JOIN sales_invoices s ON s.invoice_id = i.id
    CROSS JOIN (SELECT tax_rate, rounding_mode FROM settings LIMIT 1) cfg
    WHERE i.shift_id = sqlc.arg(report_shift_id)
)
SELECT
    COALESCE((SELECT invoice_code FROM invoices AS first_i WHERE first_i.shift_id = sqlc.arg(report_shift_id) ORDER BY first_i.id LIMIT 1), '') AS first_invoice,
    COALESCE((SELECT invoice_code FROM invoices AS last_i WHERE last_i.shift_id = sqlc.arg(report_shift_id) ORDER BY last_i.id DESC LIMIT 1), '') AS last_invoice,
    (SELECT COUNT(*) FROM invoices AS transaction_i WHERE transaction_i.shift_id = sqlc.arg(report_shift_id)) AS transactions,
    COALESCE((
        SELECT SUM(CASE WHEN ip.quantity > 0 THEN ip.quantity ELSE 0 END)
        FROM invoice_products ip
        JOIN invoices i ON i.id = ip.invoice_id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS items_sold,
    (
        SELECT COUNT(*)
        FROM return_invoices r
        JOIN invoices i ON i.id = r.invoice_id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ) AS returns,
    COALESCE((
        SELECT SUM(i.subtotal)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS gross_sales,
    COALESCE((
        SELECT SUM(i.subtotal - i.discounted_total)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS discounts,
    COALESCE((
        SELECT SUM(MAX(p.price - ip.unit_price, 0) * ip.quantity)
        FROM invoice_products ip
        JOIN products p ON p.id = ip.product_id
        JOIN invoices i ON i.id = ip.invoice_id
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS price_deductions,
    COALESCE((
        SELECT -SUM(CASE WHEN ip.line_total < 0 THEN ip.line_total ELSE 0 END)
        FROM invoice_products ip
        JOIN invoices i ON i.id = ip.invoice_id
        JOIN return_invoices r ON r.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) AS return_amount,
    COALESCE((SELECT tax FROM shift_tax), 0) AS tax,
    -- Cash-rounding adjustment: checkout snaps the grand total to the
    -- rounding currency's cash grid, so grand_total - discounted_total is
    -- tax PLUS that snap. Reporting the whole difference as "tax" made the
    -- line drift every shift (and go negative) even at a 0% rate, so the
    -- snap gets its own line.
    COALESCE((
        SELECT SUM(i.grand_total - i.discounted_total)
        FROM invoices i
        JOIN sales_invoices s ON s.invoice_id = i.id
        WHERE i.shift_id = sqlc.arg(report_shift_id)
    ), 0) - COALESCE((SELECT tax FROM shift_tax), 0) AS cash_rounding,
    COALESCE((SELECT SUM(net_i.grand_total) FROM invoices AS net_i WHERE net_i.shift_id = sqlc.arg(report_shift_id)), 0)
    -- "Calculate on checkout" coupon types are settled later (e.g. by the
    -- coupon issuer) rather than collected by the branch now, so their
    -- payment amount is carried by the invoice's grand_total but must not
    -- count as net sales for this shift.
    - COALESCE((
        SELECT SUM(icp.amount)
        FROM invoice_coupon_payments icp
        JOIN coupon_types ct ON ct.id = icp.coupon_type_id
        JOIN invoices i ON i.id = icp.invoice_id
        WHERE i.shift_id = sqlc.arg(report_shift_id) AND ct.calculate_on_checkout = 1
    ), 0) AS net_sales;

-- name: CreateShiftBalanceReplacement :exec
INSERT INTO shift_balance_replacements (shift_id, from_cashbox_id, to_cashbox_id, amount, created_at)
VALUES (?, ?, ?, ?, datetime('now'));

-- name: ListClosedShifts :many
SELECT * FROM shifts WHERE is_closed = 1 ORDER BY id;
