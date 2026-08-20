-- Generalizes currency conversion/rounding away from the hardcoded LBP
-- 10,000 rule so shops can add arbitrary currencies with their own rate and
-- cash-rounding unit.
--
-- units_per_usd_micros: how many micro-units (x1,000,000) of this currency
-- equal one USD. A single integer column that represents both weaker and
-- stronger-than-USD currencies without floats.
--
-- cash_rounding_unit: physical cash in this currency rounds to the nearest
-- multiple of this, expressed in the currency's own major units.
ALTER TABLE currencies ADD COLUMN units_per_usd_micros bigint NOT NULL DEFAULT 1000000
  CHECK (units_per_usd_micros > 0);
ALTER TABLE currencies ADD COLUMN cash_rounding_unit bigint NOT NULL DEFAULT 1
  CHECK (cash_rounding_unit > 0);

-- Default currency (USC, the accounting cent) and USD are fixed points of
-- the existing accounting model: 100 USC per USD, whole-dollar cash
-- rounding.
UPDATE currencies SET units_per_usd_micros = 100000000, cash_rounding_unit = 1
  WHERE code = 'USC';
UPDATE currencies SET units_per_usd_micros = 1000000, cash_rounding_unit = 1
  WHERE code = 'USD';

-- Backfill LBP's rate from the existing single global settings.exchange_rate
-- (LBP per USD), which was the only place this rate actually lived until
-- now. Falls back to the historical default of 90000 if settings is empty.
UPDATE currencies SET
  units_per_usd_micros = (SELECT COALESCE(MAX(exchange_rate), 90000) FROM settings) * 1000000,
  cash_rounding_unit = 10000
  WHERE code = 'LBP';
