-- Product codes whose price and discount the cashier may edit by hand while
-- the price_edit_restricted flag is on. Comma-separated, case-insensitive.
ALTER TABLE settings ADD COLUMN price_edit_codes TEXT NOT NULL DEFAULT '';
