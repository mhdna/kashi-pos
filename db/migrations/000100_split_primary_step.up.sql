-- 000099 deliberately gave the split no denomination of its own, reusing the
-- primary currency's cash_rounding_unit as the split point. In practice that
-- couples two different things: cash_rounding_unit also drives the headline
-- total's own rounding when a currency is selected on its own (see
-- formatAmountInCurrency in Calculator.vue) -- rounding that matters on its
-- own terms, e.g. LBP genuinely can't make change finer than its smallest
-- note. Raising a currency's cash_rounding_unit just to get a coarser split
-- (USD's hardcoded-10 behaviour, pre-000099) now also rounds its standalone
-- total, which never happened before -- USD was exact whether or not the
-- split gave whole tens.
--
-- split_primary_step restores that independence: it is the split's own
-- denomination, separate from cash_rounding_unit. Zero means "no override",
-- falling back to the primary currency's cash_rounding_unit exactly as
-- 000099 left it, so a till that never sets this keeps behaving the same.
ALTER TABLE settings ADD COLUMN split_primary_step integer NOT NULL DEFAULT 0
  CHECK (split_primary_step >= 0);
