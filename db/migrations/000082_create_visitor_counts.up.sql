-- Per-day people/visitor tally driven by the nav-drawer up/down counter
-- (see the `visitor_counter` feature flag). One row per local calendar day;
-- `count` is clamped at 0 by the AdjustVisitorCount query. Rows are kept
-- even when the counter is disabled -- the flag only hides the UI and the
-- monthly-income chip, it never deletes history.
CREATE TABLE IF NOT EXISTS visitor_counts (
    day text primary key,
    count integer not null default 0,
    updated_at text not null default (datetime('now', 'localtime'))
);
