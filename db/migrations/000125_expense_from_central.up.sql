-- An expense central paid for on the branch's behalf (من المركزي). It's
-- still the branch's expense, but the money never left this drawer, so the
-- shift's expected cash doesn't subtract it (see ListExpensesForShift).
-- Chosen when the expense is added, or flipped later by an admin here or
-- from kashi.
ALTER TABLE expenses ADD COLUMN from_central integer not null default 0;
