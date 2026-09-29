-- The totals panel shows a two-currency split ("$40 + LL 439,000"). Until now
-- only its second half was configurable, as settings.rounding_currency; the
-- first half was hardcoded to USD in Calculator.vue, and the split point was a
-- literal 10 -- whole tens of dollars, remainder in the rounding currency.
--
-- split_primary_currency makes the first half a setting like the second, and
-- split_primary_first swaps the two for a branch that quotes local currency
-- first.
--
-- There is deliberately no column for the split point. It is the primary
-- currency's own cash_rounding_unit (000052), which is what the hardcoded 10
-- always stood for: split at the largest denomination that currency actually
-- rounds to. A currency whose unit is 1 splits at 1, which is the same thing
-- as not splitting -- and that case already collapses to a single line.
--
-- Empty default rather than 'USD': an empty value means "fall back to the
-- default currency", so a till that never configures this behaves exactly as
-- it does today.
ALTER TABLE settings ADD COLUMN split_primary_currency text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN split_primary_first integer NOT NULL DEFAULT 1
  CHECK (split_primary_first IN (0, 1));
