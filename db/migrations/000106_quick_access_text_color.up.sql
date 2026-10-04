-- Optional text colour for a quick-access card, overriding the automatic
-- black/white pick against its background ('' = automatic). #rrggbb.
ALTER TABLE quick_access_items ADD COLUMN text_color text NOT NULL DEFAULT '';
