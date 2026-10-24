-- Which look the printed invoice has: a theme (fonts, dividers, total block)
-- and one of its layouts (how the item lines are arranged). Ids, not
-- display names; the catalog lives in receipt_theme.go and unknown ids fall
-- back to the default there. Defaults are the original look with the four
-- column layout. Kashi sets both from its branch settings.
ALTER TABLE settings ADD COLUMN receipt_theme TEXT NOT NULL DEFAULT 'classic';
ALTER TABLE settings ADD COLUMN receipt_layout TEXT NOT NULL DEFAULT 'A';
