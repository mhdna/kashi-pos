-- Completeness check (see sync_full_check.go). remote_updated_at is kashi's
-- updated_at for the version of the row this till last applied, so a check
-- can tell "kashi has a newer copy" from "up to date" without refetching
-- everything. NULL = never applied from kashi (or applied before this
-- column existed), which the check treats as stale.
ALTER TABLE clients ADD COLUMN remote_updated_at text;
ALTER TABLE products ADD COLUMN remote_updated_at text;

-- identity_adopted_at: when this till last adopted kashi's client uids for
-- the clients it had linked (see adoptBranchClientUIDs). NULL = never.
ALTER TABLE sync_state ADD COLUMN identity_adopted_at text;
ALTER TABLE sync_state ADD COLUMN last_full_check_at text;
ALTER TABLE sync_state ADD COLUMN last_full_check_json text NOT NULL DEFAULT '';
