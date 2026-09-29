-- Lets a historical shift-close pulled down from kashi (see
-- sync_branch_history.go) be reattached to the exact branch_shifts row it
-- came from, independent of local autoincrement ids -- which restart at 1
-- on a freshly re-provisioned till and would otherwise have no relation to
-- the id kashi remembers for that same close. Nullable: every shift closed
-- locally in the ordinary way has no ref at all, same as
-- expenses/loans.client_ref being unique-but-not-required elsewhere.
ALTER TABLE shifts ADD COLUMN client_ref TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_shifts_client_ref
    ON shifts(client_ref) WHERE client_ref IS NOT NULL;
