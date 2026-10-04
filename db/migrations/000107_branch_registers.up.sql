-- A kashi branch can run several tills ("registers"). This till logs in
-- with its register code (settings.branch_code, also its invoice prefix),
-- which differs from its branch's code once the branch has more than one
-- till. kashi reports the branch code back on every settings push; it's
-- what marks this branch's own rows in the branch_stock mirror. '' = not
-- learned yet (treated as branch_code, the single-till case).
ALTER TABLE settings ADD COLUMN home_branch_code text NOT NULL DEFAULT '';

-- What checkout does when a line needs more stock than the branch has:
-- allow (sell), warn (cashier confirms) or block (refuse). Set by kashi per
-- branch via update_settings. kashi flags any sale that drives stock
-- negative whatever the mode, since this till's view of stock can be stale.
ALTER TABLE settings ADD COLUMN oversell_mode text NOT NULL DEFAULT 'allow'
    CHECK (oversell_mode IN ('allow', 'warn', 'block'));

-- Branch-wide target progress as kashi last reported it: every register's
-- synced sales in each target's range. fetched_at (UTC, sqlite datetime
-- format) is when this till fetched it; its own sales kashi hadn't received
-- by then are added on top when showing progress.
CREATE TABLE IF NOT EXISTS branch_target_progress (
    target_id integer PRIMARY KEY,
    achieved_cents integer NOT NULL,
    fetched_at text NOT NULL
);
