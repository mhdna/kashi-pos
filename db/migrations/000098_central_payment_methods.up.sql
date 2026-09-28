-- Payment methods and currencies come from kashi, and a till no longer has
-- any of its own.
--
-- cashbox_accounts gains is_active, mirroring the column kashi's table has
-- always had. It is a flag rather than a delete because the row is the FK
-- target of invoice payments, expenses and shift balances (000005, 000011,
-- 000074): deleting a method that has ever been used would orphan the history
-- that explains how a shift balanced. Deactivating hides it from checkout and
-- leaves every past invoice still readable.
--
-- Everything starts active. The first sync after this migration is what
-- decides: kashi sends its whole set, and applySyncedCashboxAccounts
-- deactivates whatever is not in it.
ALTER TABLE cashbox_accounts ADD COLUMN is_active integer NOT NULL DEFAULT 1;

-- "On Account" and "On Coupon" are not payment methods -- they are internal
-- bookkeeping placeholders that the client-account and coupon settlement
-- paths look up by exact name (see shifts.go and currencies.go). kashi has no
-- concept of them and never will, so they are exempt from the reconcile and
-- pinned active here to make that explicit rather than implicit.
UPDATE cashbox_accounts SET is_active = 1
WHERE name IN ('On Account', 'On Coupon');
