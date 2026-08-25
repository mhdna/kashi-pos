-- Keep the POS money setup limited to USD and LBP. Deactivation preserves
-- historical invoices while removing other currencies from new selections.
UPDATE currencies
SET is_active = CASE WHEN code IN ('USD', 'LBP') THEN 1 ELSE 0 END;

UPDATE currencies
SET value_in_default_currency = 9000000
WHERE code = 'LBP';

UPDATE settings
SET exchange_rate = 90000
WHERE exchange_rate IS NULL OR exchange_rate <> 90000;

-- The two special rows are internal bookkeeping methods and remain hidden
-- from checkout. The four rows below are the only selectable methods.
UPDATE cashbox_accounts SET name = 'Cash $$', color = '#2E7D32'
WHERE name = 'Cash USD' AND currency_code = 'USD';
UPDATE cashbox_accounts SET color = '#1565C0'
WHERE name = 'Cash LBP' AND currency_code = 'LBP';
UPDATE cashbox_accounts SET color = '#7B1FA2'
WHERE name = 'Visa' AND currency_code = 'USD';
UPDATE cashbox_accounts SET color = '#D32F2F'
WHERE name = 'Whish' AND currency_code = 'USD';

-- Remove obsolete user-created payment methods, but retain internal account
-- and coupon settlement methods used by client-accounting flows. Also
-- skip any account still referenced by historical data (invoices,
-- payments, shift balances, expenses, loans) -- deleting those would
-- fail the foreign key constraint outright, and a shop with real history
-- shouldn't have its past records orphaned just to tidy the method list.
DELETE FROM cashbox_accounts
WHERE name NOT IN ('Cash $$', 'Cash USD', 'Cash LBP', 'Visa', 'Whish', 'On Account', 'On Coupon')
  AND NOT EXISTS (SELECT 1 FROM invoices WHERE invoices.cashbox_id = cashbox_accounts.id)
  AND NOT EXISTS (SELECT 1 FROM invoice_payments WHERE invoice_payments.cashbox_id = cashbox_accounts.id)
  AND NOT EXISTS (SELECT 1 FROM invoice_payment_entries WHERE invoice_payment_entries.cashbox_id = cashbox_accounts.id)
  AND NOT EXISTS (SELECT 1 FROM client_account_payments WHERE client_account_payments.cashbox_id = cashbox_accounts.id)
  AND NOT EXISTS (
    SELECT 1 FROM shift_balance_replacements
    WHERE shift_balance_replacements.from_cashbox_id = cashbox_accounts.id
       OR shift_balance_replacements.to_cashbox_id = cashbox_accounts.id
  )
  AND NOT EXISTS (SELECT 1 FROM shifts_accounts_balances WHERE shifts_accounts_balances.account_id = cashbox_accounts.id)
  AND NOT EXISTS (SELECT 1 FROM expenses WHERE expenses.cashbox_account_id = cashbox_accounts.id)
  AND NOT EXISTS (SELECT 1 FROM loans WHERE loans.cashbox_account_id = cashbox_accounts.id);
