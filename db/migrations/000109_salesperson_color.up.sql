-- Avatar color, set in kashi (a random one per salesperson, changeable there)
-- and pulled down with the roster. Empty for a till-only salesperson; the UI
-- then derives a stable color from the name.
ALTER TABLE salespersons ADD COLUMN color TEXT NOT NULL DEFAULT '';
