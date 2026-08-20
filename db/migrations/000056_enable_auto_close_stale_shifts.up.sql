-- The shop owner explicitly asked for open shifts from a prior calendar day
-- to be auto-closed on startup/login, unlike migration 53 which deliberately
-- left this off for existing installs by default. Turn it on for everyone
-- upgrading through this migration; it remains a Settings toggle a shop can
-- turn back off if they don't want it.
INSERT INTO feature_flags (key, enabled)
VALUES ('auto_close_stale_shifts', 1)
ON CONFLICT (key) DO UPDATE SET enabled = 1;
