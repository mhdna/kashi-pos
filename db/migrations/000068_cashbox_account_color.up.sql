-- Lets an admin pick a custom accent color for a payment method, shown on
-- its button in the checkout calculator. Empty string means "no custom
-- color" - the frontend falls back to its existing name-based heuristic.
ALTER TABLE cashbox_accounts ADD COLUMN color TEXT NOT NULL DEFAULT '';
