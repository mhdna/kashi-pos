CREATE TABLE IF NOT EXISTS feature_flags (
    key TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1))
);

INSERT OR IGNORE INTO feature_flags (key, enabled)
VALUES ('custom_item_discounts', 0);
