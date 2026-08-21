-- Default opening cash float applied to every shift when it's opened
-- (snapshotted onto the shift itself, see 000064), so the closing
-- reconciliation's "expected" total accounts for cash that was already in
-- the drawer at shift start rather than assuming it starts at zero.
ALTER TABLE settings ADD COLUMN default_opening_float_usd bigint NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN default_opening_float_lbp bigint NOT NULL DEFAULT 0;
