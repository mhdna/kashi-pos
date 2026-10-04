-- Optional background colour an admin picks for each quick-access card from
-- Settings ('' = the default card colour). Stored as #rrggbb.
ALTER TABLE quick_access_items ADD COLUMN color text NOT NULL DEFAULT '';
