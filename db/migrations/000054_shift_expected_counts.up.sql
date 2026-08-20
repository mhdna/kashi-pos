-- Persists what was *expected* at close time alongside the existing
-- counted_* columns, so a monthly "expected vs counted" variance can be
-- shown without ever having to recompute history under later-changed
-- exchange-rate/rounding settings.
ALTER TABLE shifts ADD COLUMN expected_usd bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN expected_lbp bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN expected_visa bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN expected_whish bigint NOT NULL DEFAULT 0;
