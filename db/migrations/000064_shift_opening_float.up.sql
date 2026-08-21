-- Snapshot of the opening cash float for this shift, set from the settings
-- default when the shift is opened and adjustable afterwards (manager-only)
-- via "apply to current shift". Feeds into expected_usd/expected_lbp at
-- close time so a shift that refunds more than it sold doesn't show a false
-- overage just because counted cash can't go negative.
ALTER TABLE shifts ADD COLUMN opening_float_usd bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN opening_float_lbp bigint NOT NULL DEFAULT 0;
