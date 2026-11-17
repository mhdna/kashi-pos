-- A return of a sale sold On Account settles against what the client owes on
-- that sale (refund to the On Account method), not in cash.

CREATE INDEX IF NOT EXISTS idx_return_invoices_sale ON return_invoices(sales_invoice_id);

-- client_account_lines: each on-account sale with what returns of it took off
-- (their On Account refunds, negative) already netted into amount.
CREATE VIEW IF NOT EXISTS client_account_lines AS
SELECT cae.id, cae.client_id, cae.invoice_id,
       cae.amount + COALESCE(rc.returned, 0) AS amount,
       cae.created_at
FROM client_account_entries cae
LEFT JOIN (
    SELECT r.sales_invoice_id, SUM(ip.amount) AS returned
    FROM return_invoices r
    JOIN invoice_payments ip ON ip.invoice_id = r.invoice_id
    JOIN cashbox_accounts ca ON ca.id = ip.cashbox_id
    WHERE lower(trim(ca.name)) = 'on account'
    GROUP BY r.sales_invoice_id
) rc ON rc.sales_invoice_id = cae.invoice_id;

-- Before this, a refund could not go to On Account, so returns of unpaid
-- on-account sales were recorded as a cash refund nobody was given. Move those
-- to On Account: only a sale with no payment back yet, refunded in one method,
-- with its refunds no bigger than the sale's on-account amount.
CREATE TEMP TABLE on_account_return_fix AS
SELECT r.invoice_id AS return_id, ip.cashbox_id AS old_account, ip.amount AS amount,
       i.shift_id AS shift_id, oa.id AS on_account
FROM return_invoices r
JOIN client_account_entries cae ON cae.invoice_id = r.sales_invoice_id
JOIN invoices i ON i.id = r.invoice_id
JOIN invoice_payments ip ON ip.invoice_id = r.invoice_id
JOIN (SELECT id FROM cashbox_accounts WHERE lower(trim(name)) = 'on account' ORDER BY id LIMIT 1) oa
WHERE ip.amount < 0
  AND ip.cashbox_id <> oa.id
  AND (SELECT COUNT(*) FROM invoice_payments x WHERE x.invoice_id = r.invoice_id) = 1
  AND NOT EXISTS (SELECT 1 FROM client_account_payments cap WHERE cap.invoice_id = cae.invoice_id)
  AND (SELECT -SUM(ip2.amount) FROM return_invoices r2
       JOIN invoice_payments ip2 ON ip2.invoice_id = r2.invoice_id
       WHERE r2.sales_invoice_id = r.sales_invoice_id) <= cae.amount;

-- The refund never left the drawer: give it back to the old method's shift
-- balance and take it off On Account's.
UPDATE shifts_accounts_balances
SET balance = balance - (SELECT SUM(f.amount) FROM on_account_return_fix f
                         WHERE f.old_account = shifts_accounts_balances.account_id
                           AND f.shift_id = shifts_accounts_balances.shift_id)
WHERE EXISTS (SELECT 1 FROM on_account_return_fix f
              WHERE f.old_account = shifts_accounts_balances.account_id
                AND f.shift_id = shifts_accounts_balances.shift_id);

INSERT OR IGNORE INTO shifts_accounts_balances (account_id, shift_id, balance)
SELECT DISTINCT on_account, shift_id, 0 FROM on_account_return_fix;

UPDATE shifts_accounts_balances
SET balance = balance + (SELECT SUM(f.amount) FROM on_account_return_fix f
                         WHERE f.on_account = shifts_accounts_balances.account_id
                           AND f.shift_id = shifts_accounts_balances.shift_id)
WHERE EXISTS (SELECT 1 FROM on_account_return_fix f
              WHERE f.on_account = shifts_accounts_balances.account_id
                AND f.shift_id = shifts_accounts_balances.shift_id);

UPDATE invoice_payment_entries
SET cashbox_id = (SELECT f.on_account FROM on_account_return_fix f WHERE f.return_id = invoice_payment_entries.invoice_id)
WHERE invoice_id IN (SELECT return_id FROM on_account_return_fix)
  AND cashbox_id = (SELECT f.old_account FROM on_account_return_fix f WHERE f.return_id = invoice_payment_entries.invoice_id);

UPDATE invoice_payments
SET cashbox_id = (SELECT f.on_account FROM on_account_return_fix f WHERE f.return_id = invoice_payments.invoice_id)
WHERE invoice_id IN (SELECT return_id FROM on_account_return_fix);

DROP TABLE on_account_return_fix;
