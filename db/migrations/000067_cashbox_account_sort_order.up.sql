-- Lets an admin control the display order of payment methods (cashbox
-- accounts) in Settings and the checkout calculator, instead of the
-- previous implicit "whatever order they were created in" (id ASC).
ALTER TABLE cashbox_accounts ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
UPDATE cashbox_accounts SET sort_order = id;

-- The two seeded cash accounts were historically named literally "USD"/"LBP"
-- and had their displayed label hardcoded to "Cash" based on sort position
-- (whichever two rows happened to sort first). Renaming them here removes
-- the need for that position-dependent hack, which admin reordering would
-- otherwise break.
UPDATE cashbox_accounts SET name = 'Cash USD' WHERE name = 'USD' AND currency_code = 'USD';
UPDATE cashbox_accounts SET name = 'Cash LBP' WHERE name = 'LBP' AND currency_code = 'LBP';
