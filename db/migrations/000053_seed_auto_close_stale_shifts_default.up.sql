-- Enables the auto_close_stale_shifts feature flag by default. Only ever
-- applied for brand-new databases (see migrations.go's special case for
-- this migration version) - an existing install upgrading through this
-- migration keeps whatever behavior it already had, since silently
-- flipping shift auto-close on for a shop that never asked for it could
-- surprise them.
INSERT OR IGNORE INTO feature_flags (key, enabled)
VALUES ('auto_close_stale_shifts', 1);
