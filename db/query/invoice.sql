-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = ?;

-- name: ListInvoicePayments :many
SELECT * FROM invoice_payments WHERE invoice_id = ? ORDER BY cashbox_id;

-- name: ListInvoicePaymentEntries :many
SELECT * FROM invoice_payment_entries WHERE invoice_id = ? ORDER BY id;

-- name: DeleteInvoicePayments :exec
DELETE FROM invoice_payments WHERE invoice_id = ?;

-- name: DeleteInvoicePaymentEntries :exec
DELETE FROM invoice_payment_entries WHERE invoice_id = ?;

-- name: UpdateInvoiceClient :exec
UPDATE invoices SET client_id = ? WHERE id = ?;

-- name: UpdateInvoiceSalesperson :exec
UPDATE invoices SET salesperson_id = ? WHERE id = ?;

-- name: UpdateInvoiceCashbox :exec
UPDATE invoices SET cashbox_id = ? WHERE id = ?;

-- name: UpdateInvoiceGrandTotal :exec
UPDATE invoices SET grand_total = ? WHERE id = ?;

-- name: UpdateInvoicePaymentAmount :exec
UPDATE invoice_payments SET amount = ? WHERE invoice_id = ? AND cashbox_id = ?;

-- name: UpdateInvoicePaymentEntryAmount :exec
UPDATE invoice_payment_entries SET amount = ? WHERE id = ?;

-- name: GetSalesInvoiceByCode :one
SELECT i.*
FROM invoices i
JOIN sales_invoices s ON s.invoice_id = i.id
WHERE lower(i.invoice_code) = lower(?);

-- name: GetSalesInvoiceByID :one
SELECT i.*
FROM invoices i
JOIN sales_invoices s ON s.invoice_id = i.id
WHERE i.id = ?;

-- name: GetReturnedQuantityForProduct :one
SELECT COALESCE(SUM(CASE WHEN ip.quantity < 0 THEN -ip.quantity ELSE 0 END), 0) AS quantity
FROM return_invoices r
JOIN invoice_products ip ON ip.invoice_id = r.invoice_id
WHERE r.sales_invoice_id = ? AND ip.product_id = ?;

-- name: GetReturnedQuantityForProductPrice :one
SELECT COALESCE(SUM(CASE WHEN ip.quantity < 0 THEN -ip.quantity ELSE 0 END), 0) AS quantity
FROM return_invoices r
JOIN invoice_products ip ON ip.invoice_id = r.invoice_id
WHERE r.sales_invoice_id = ? AND ip.product_id = ? AND ip.unit_price = ? AND ip.discount = ?;

-- name: ListInvoices :many
SELECT * FROM invoices ORDER BY id;

-- name: ListReturnInvoices :many
SELECT invoice_id, sales_invoice_id FROM return_invoices;

-- name: ListInvoicesPage :many
SELECT i.*, COALESCE(s.full_name, 'Anonymous') AS salesperson_name
FROM invoices i
LEFT JOIN salespersons s ON s.id = i.salesperson_id
LEFT JOIN clients c ON c.id = i.client_id
WHERE (COALESCE(sqlc.arg(invoice_filter), '') = '' OR lower(i.invoice_code) LIKE '%' || lower(sqlc.arg(invoice_filter)) || '%')
  AND (COALESCE(sqlc.arg(customer_filter), '') = '' OR lower(COALESCE(c.name, 'Walk-in customer')) LIKE '%' || lower(sqlc.arg(customer_filter)) || '%')
  AND (COALESCE(sqlc.arg(salesperson_filter), '') = '' OR lower(COALESCE(s.full_name, 'Anonymous')) LIKE '%' || lower(sqlc.arg(salesperson_filter)) || '%')
  AND (COALESCE(sqlc.arg(date_filter), '') = '' OR lower(datetime(i.created_at, 'localtime')) LIKE '%' || lower(sqlc.arg(date_filter)) || '%')
  AND (COALESCE(sqlc.arg(total_filter), '') = '' OR printf('%.2f', i.grand_total / 100.0) LIKE '%' || trim(replace(sqlc.arg(total_filter), '$', '')) || '%')
  AND (
    COALESCE(sqlc.arg(paid_to_filter), '') = ''
    OR EXISTS (
      SELECT 1
      FROM invoice_payments ip
      JOIN cashbox_accounts ca ON ca.id = ip.cashbox_id
      WHERE ip.invoice_id = i.id
        AND ip.amount > 0
        AND lower(ca.name || ' ' || ca.currency_code) LIKE '%' || lower(sqlc.arg(paid_to_filter)) || '%'
    )
    OR (
      NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id AND ip.amount > 0)
      AND EXISTS (
        SELECT 1
        FROM cashbox_accounts ca
        WHERE ca.id = i.cashbox_id
          AND lower(ca.name || ' ' || ca.currency_code) LIKE '%' || lower(sqlc.arg(paid_to_filter)) || '%'
      )
    )
  )
ORDER BY i.id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: CountFilteredInvoices :one
SELECT COUNT(*)
FROM invoices i
LEFT JOIN salespersons s ON s.id = i.salesperson_id
LEFT JOIN clients c ON c.id = i.client_id
WHERE (COALESCE(sqlc.arg(invoice_filter), '') = '' OR lower(i.invoice_code) LIKE '%' || lower(sqlc.arg(invoice_filter)) || '%')
  AND (COALESCE(sqlc.arg(customer_filter), '') = '' OR lower(COALESCE(c.name, 'Walk-in customer')) LIKE '%' || lower(sqlc.arg(customer_filter)) || '%')
  AND (COALESCE(sqlc.arg(salesperson_filter), '') = '' OR lower(COALESCE(s.full_name, 'Anonymous')) LIKE '%' || lower(sqlc.arg(salesperson_filter)) || '%')
  AND (COALESCE(sqlc.arg(date_filter), '') = '' OR lower(datetime(i.created_at, 'localtime')) LIKE '%' || lower(sqlc.arg(date_filter)) || '%')
  AND (COALESCE(sqlc.arg(total_filter), '') = '' OR printf('%.2f', i.grand_total / 100.0) LIKE '%' || trim(replace(sqlc.arg(total_filter), '$', '')) || '%')
  AND (
    COALESCE(sqlc.arg(paid_to_filter), '') = ''
    OR EXISTS (
      SELECT 1
      FROM invoice_payments ip
      JOIN cashbox_accounts ca ON ca.id = ip.cashbox_id
      WHERE ip.invoice_id = i.id
        AND ip.amount > 0
        AND lower(ca.name || ' ' || ca.currency_code) LIKE '%' || lower(sqlc.arg(paid_to_filter)) || '%'
    )
    OR (
      NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id AND ip.amount > 0)
      AND EXISTS (
        SELECT 1
        FROM cashbox_accounts ca
        WHERE ca.id = i.cashbox_id
          AND lower(ca.name || ' ' || ca.currency_code) LIKE '%' || lower(sqlc.arg(paid_to_filter)) || '%'
      )
    )
  );

-- name: ListInvoicesByDate :many
SELECT i.*, COALESCE(s.full_name, 'Anonymous') AS salesperson_name
FROM invoices i
LEFT JOIN salespersons s ON s.id = i.salesperson_id
WHERE date(i.created_at, 'localtime') = sqlc.arg(date)
ORDER BY i.id DESC;

-- name: ListSalespersonProductSales :many
SELECT COALESCE(sp.id, 0) AS salesperson_id,
       COALESCE(sp.full_name, 'Anonymous') AS salesperson_name,
       p.id AS product_id, p.barcode, p.code, p.name, p.color, p.size, p.currency_code,
       CAST(COALESCE(SUM(ip.quantity), 0) AS INTEGER) AS quantity,
       CAST(COALESCE(SUM(ip.line_total), 0) AS INTEGER) AS revenue
FROM sales_invoices si
JOIN invoices i ON i.id = si.invoice_id
JOIN invoice_products ip ON ip.invoice_id = i.id
JOIN products p ON p.id = ip.product_id
LEFT JOIN salespersons sp ON sp.id = COALESCE(NULLIF(ip.salesperson_id, 0), NULLIF(i.salesperson_id, 0))
WHERE date(i.created_at, 'localtime') >= sqlc.arg(report_from)
  AND date(i.created_at, 'localtime') <= sqlc.arg(report_to)
GROUP BY sp.id, sp.full_name, p.id, p.barcode, p.code, p.name, p.color, p.size, p.currency_code
ORDER BY sp.full_name, SUM(ip.quantity) DESC, p.name, p.id;

-- name: ListSalespersonSalesSummary :many
-- Per-salesperson totals from original sale invoices only (return/exchange
-- invoices are attributed separately by ListSalespersonReturnSummary, and
-- exchange invoices only ever carry a net difference so they don't belong
-- in gross/items counts here). Same line-then-invoice salesperson fallback
-- as ListSalespersonProductSales.
SELECT COALESCE(sp.id, 0) AS salesperson_id,
       COALESCE(sp.full_name, 'Anonymous') AS salesperson_name,
       CAST(COUNT(DISTINCT i.id) AS INTEGER) AS invoice_count,
       CAST(COALESCE(SUM(ip.quantity), 0) AS INTEGER) AS items_sold,
       CAST(COALESCE(SUM(ip.line_total), 0) AS INTEGER) AS gross_amount
FROM sales_invoices si
JOIN invoices i ON i.id = si.invoice_id
JOIN invoice_products ip ON ip.invoice_id = i.id
LEFT JOIN salespersons sp ON sp.id = COALESCE(NULLIF(ip.salesperson_id, 0), NULLIF(i.salesperson_id, 0))
WHERE date(i.created_at, 'localtime') >= sqlc.arg(report_from)
  AND date(i.created_at, 'localtime') <= sqlc.arg(report_to)
GROUP BY sp.id, sp.full_name
ORDER BY sp.full_name;

-- name: ListSalespersonReturnSummary :many
-- Per-salesperson totals from return/exchange invoices. adjustment_amount is
-- the signed net effect (negative returned lines + positive replacement
-- lines) to fold into net sales; return_amount is the unsigned magnitude of
-- only the returned lines, used for the return-rate performance penalty.
SELECT COALESCE(sp.id, 0) AS salesperson_id,
       CAST(COALESCE(SUM(ip.line_total), 0) AS INTEGER) AS adjustment_amount,
       CAST(COALESCE(SUM(CASE WHEN ip.line_total < 0 THEN -ip.line_total ELSE 0 END), 0) AS INTEGER) AS return_amount
FROM return_invoices ri
JOIN invoices i ON i.id = ri.invoice_id
JOIN invoice_products ip ON ip.invoice_id = i.id
LEFT JOIN salespersons sp ON sp.id = COALESCE(NULLIF(ip.salesperson_id, 0), NULLIF(i.salesperson_id, 0))
WHERE date(i.created_at, 'localtime') >= sqlc.arg(report_from)
  AND date(i.created_at, 'localtime') <= sqlc.arg(report_to)
GROUP BY sp.id;

-- name: ListSalespersonDiscountSummary :many
-- Revenue-weighted discount total from original sale invoices, attributed at
-- the invoice level (discount is an invoice-wide concept, unlike line-level
-- item sales attribution). Exchange invoices are excluded since their
-- subtotal/grandTotal only ever reflect the net adjustment, not a discount.
SELECT COALESCE(sp.id, 0) AS salesperson_id,
       CAST(COALESCE(SUM(i.subtotal - i.grand_total), 0) AS INTEGER) AS discount_amount,
       CAST(COALESCE(SUM(i.subtotal), 0) AS INTEGER) AS subtotal_total
FROM sales_invoices si
JOIN invoices i ON i.id = si.invoice_id
LEFT JOIN salespersons sp ON sp.id = NULLIF(i.salesperson_id, 0)
WHERE date(i.created_at, 'localtime') >= sqlc.arg(report_from)
  AND date(i.created_at, 'localtime') <= sqlc.arg(report_to)
GROUP BY sp.id;

-- name: CountInvoices :one
SELECT COUNT(*) FROM invoices;

-- name: ListInvoicesByShiftID :many
SELECT * FROM invoices WHERE shift_id = ? ORDER BY id;

-- name: CreateInvoice :one
INSERT INTO invoices (
    cashbox_id, shift_id, invoice_code, invoice_index,
    year, client_id, inventory_id, discount,
    subtotal, discounted_total, grand_total, salesperson_id, notes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetInvoiceProduct :one
SELECT * FROM invoice_products WHERE invoice_id = ? AND product_id = ?;

-- name: ListInvoiceProducts :many
SELECT * FROM invoice_products WHERE invoice_id = ?;

-- name: CreateInvoiceProduct :exec
INSERT INTO invoice_products (invoice_id, product_id, unit_price, line_total, discount, quantity, salesperson_id, note, price_list_id, discount_list_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateInvoicePayment :exec
INSERT INTO invoice_payments (invoice_id, cashbox_id, amount) VALUES (?, ?, ?)
ON CONFLICT (invoice_id, cashbox_id) DO UPDATE SET amount = invoice_payments.amount + excluded.amount;

-- name: CreateInvoicePaymentEntry :exec
INSERT INTO invoice_payment_entries (invoice_id, cashbox_id, amount, entry_type)
VALUES (?, ?, ?, ?);

-- name: DeleteInvoiceProducts :exec
DELETE FROM invoice_products WHERE invoice_id = ?;

-- name: DeleteSalesInvoice :exec
DELETE FROM sales_invoices WHERE invoice_id = ?;

-- name: DeleteInvoice :exec
DELETE FROM invoices WHERE id = ?;

-- name: GetInvoiceIndex :one
SELECT * FROM invoice_indexes WHERE year = ? AND type = ?;

-- name: UpsertInvoiceIndex :exec
INSERT INTO invoice_indexes (year, last_index, type)
VALUES (?, ?, ?)
ON CONFLICT (year, type) DO UPDATE SET last_index = excluded.last_index;

-- name: CreateSalesInvoice :exec
INSERT INTO sales_invoices (invoice_id) VALUES (?);

-- name: CreateReturnInvoice :exec
INSERT INTO return_invoices (invoice_id, sales_invoice_id) VALUES (?, ?);

-- name: IncrementInvoicesIndex :one
INSERT INTO invoice_indexes (year, last_index, type)
VALUES (?, 1, ?)
ON CONFLICT (year, type) DO UPDATE SET last_index = last_index + 1
RETURNING last_index;

-- name: ListIncomeAccountTotals :many
-- income_date/income_month filtering was never actually used by any caller
-- (every call site passes shiftID only) - dropped in favor of the plain
-- shift scoping this query actually needs. On-account staleness (only
-- counting entries from "today") is applied in Go via
-- StaleOnAccountTotalForShift below - sqlc's SQLite query analyzer cannot
-- resolve the CTE column alias when a new bind parameter is compared
-- against it from inside an OR/CASE branch in this particular 5-way-UNION
-- CTE shape (confirmed not to be a real SQL syntax problem: every
-- restructuring - CASE, nested OR, splitting into a UNION of two WHERE
-- branches, an intermediate join-first CTE - hits the same
-- 'table alias "t" does not exist' error from sqlc, not from SQLite
-- itself), so the exclusion is computed in Go instead of fighting the
-- tool further.
WITH account_totals AS (
    SELECT i.created_at, i.shift_id, ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    UNION ALL
    SELECT i.created_at, i.shift_id, i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
    UNION ALL
    SELECT cap.created_at, cap.shift_id, cap.cashbox_id, cap.amount
    FROM client_account_payments cap
    UNION ALL
    SELECT sbr.created_at, sbr.shift_id, sbr.from_cashbox_id, -sbr.amount
    FROM shift_balance_replacements sbr
    UNION ALL
    SELECT sbr.created_at, sbr.shift_id, sbr.to_cashbox_id, sbr.amount
    FROM shift_balance_replacements sbr
)
SELECT ca.id, ca.name, ca.currency_code, ca.color, COALESCE(SUM(at2.amount), 0) AS total
FROM account_totals at2
JOIN cashbox_accounts ca ON ca.id = at2.cashbox_id
WHERE (sqlc.arg(income_shift_id) = 0 OR at2.shift_id = sqlc.arg(income_shift_id))
GROUP BY ca.id, ca.name, ca.currency_code, ca.color
ORDER BY ca.name;

-- name: StaleOnAccountTotalForShift :one
-- The portion of a shift's "On Account" total that was NOT recorded today -
-- subtracted in Go from ListIncomeAccountTotals's On Account row so a shift
-- left open across a day boundary doesn't drag prior days' on-account
-- activity into "today's" balance. On-account entries only ever originate
-- from invoices (paid or unpaid-in-full) or client_account_payments -
-- never from shift_balance_replacements (isNonCashBalanceAccount blocks
-- On Account as a swap source/destination).
WITH on_account_totals AS (
    SELECT i.created_at, i.shift_id, ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    UNION ALL
    SELECT i.created_at, i.shift_id, i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
    UNION ALL
    SELECT cap.created_at, cap.shift_id, cap.cashbox_id, cap.amount
    FROM client_account_payments cap
)
SELECT COALESCE(SUM(oat.amount), 0) AS total
FROM on_account_totals oat
JOIN cashbox_accounts ca ON ca.id = oat.cashbox_id
WHERE (sqlc.arg(stale_shift_id) = 0 OR oat.shift_id = sqlc.arg(stale_shift_id))
  AND lower(trim(ca.name)) = 'on account'
  AND (sqlc.arg(stale_reference_date) = '' OR date(oat.created_at, 'localtime') <> sqlc.arg(stale_reference_date));

-- name: ListInvoiceIncomeAccountTotalsByShift :many
WITH invoice_totals AS (
    SELECT ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    WHERE i.shift_id = sqlc.arg(invoice_income_shift_id)
    UNION ALL
    SELECT i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE i.shift_id = sqlc.arg(invoice_income_shift_id)
      AND NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
    UNION ALL
    SELECT sbr.from_cashbox_id, -sbr.amount
    FROM shift_balance_replacements sbr
    WHERE sbr.shift_id = sqlc.arg(invoice_income_shift_id)
    UNION ALL
    SELECT sbr.to_cashbox_id, sbr.amount
    FROM shift_balance_replacements sbr
    WHERE sbr.shift_id = sqlc.arg(invoice_income_shift_id)
)
SELECT ca.id, ca.name, ca.currency_code, COALESCE(SUM(invoice_totals.amount), 0) AS total
FROM invoice_totals
JOIN cashbox_accounts ca ON ca.id = invoice_totals.cashbox_id
GROUP BY ca.id, ca.name, ca.currency_code
ORDER BY ca.name;

-- name: ListInvoiceIncomeAccountTotalsByShiftAndDate :many
-- Like ListInvoiceIncomeAccountTotalsByShift, but also scoped to a single
-- calendar day - used for "current shift income" so a shift left open
-- across midnight (or for days) doesn't drag in prior days' activity as if
-- it were "today".
WITH invoice_totals AS (
    SELECT ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    WHERE i.shift_id = sqlc.arg(invoice_income_shift_id)
      AND date(i.created_at, 'localtime') = sqlc.arg(income_reference_date)
    UNION ALL
    SELECT i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE i.shift_id = sqlc.arg(invoice_income_shift_id)
      AND date(i.created_at, 'localtime') = sqlc.arg(income_reference_date)
      AND NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
    UNION ALL
    SELECT sbr.from_cashbox_id, -sbr.amount
    FROM shift_balance_replacements sbr
    WHERE sbr.shift_id = sqlc.arg(invoice_income_shift_id)
      AND date(sbr.created_at, 'localtime') = sqlc.arg(income_reference_date)
    UNION ALL
    SELECT sbr.to_cashbox_id, sbr.amount
    FROM shift_balance_replacements sbr
    WHERE sbr.shift_id = sqlc.arg(invoice_income_shift_id)
      AND date(sbr.created_at, 'localtime') = sqlc.arg(income_reference_date)
)
SELECT ca.id, ca.name, ca.currency_code, ca.color, COALESCE(SUM(invoice_totals.amount), 0) AS total
FROM invoice_totals
JOIN cashbox_accounts ca ON ca.id = invoice_totals.cashbox_id
GROUP BY ca.id, ca.name, ca.currency_code, ca.color
ORDER BY ca.name;

-- name: GetInvoiceIncomeTotalByDate :one
SELECT COALESCE(SUM(grand_total), 0)
FROM invoices i
WHERE date(i.created_at, 'localtime') = ?;

-- name: ListInvoiceIncomeAccountTotalsByDate :many
WITH invoice_totals AS (
    SELECT i.created_at AS income_date, ip.cashbox_id, ip.amount
    FROM invoices i
    JOIN invoice_payments ip ON ip.invoice_id = i.id
    WHERE date(i.created_at, 'localtime') = sqlc.arg(invoice_income_date)
    UNION ALL
    SELECT i.created_at AS income_date, i.cashbox_id, i.grand_total
    FROM invoices i
    WHERE NOT EXISTS (SELECT 1 FROM invoice_payments ip WHERE ip.invoice_id = i.id)
      AND date(i.created_at, 'localtime') = sqlc.arg(invoice_income_date)
)
SELECT ca.id, ca.name, ca.currency_code, COALESCE(SUM(invoice_totals.amount), 0) AS total
FROM invoice_totals
JOIN cashbox_accounts ca ON ca.id = invoice_totals.cashbox_id
GROUP BY ca.id, ca.name, ca.currency_code
ORDER BY ca.name;

-- name: ListClientNamesByIDs :many
SELECT id, name FROM clients WHERE id IN (sqlc.slice('ids'));

-- name: ListInvoicePaymentsByInvoiceIDs :many
SELECT invoice_id, cashbox_id, amount FROM invoice_payments WHERE invoice_id IN (sqlc.slice('ids'));
