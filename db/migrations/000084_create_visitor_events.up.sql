-- Append-only log of individual nav-drawer counter presses (see the
-- `visitor_counter` feature flag and the visitor_counts table). visitor_counts
-- stays the fast per-day read model the UI renders; this table records every
-- press as its own row so the central kashi backend can report gross
-- customers-in vs customers-out per branch, not just the clamped net tally.
--
-- A row is written only when the press actually moved the tally --
-- AdjustVisitorCountToday clamps at 0, so an "out" press against a zero
-- count changes nothing and is deliberately not logged. That keeps this
-- log reconcilable with visitor_counts: sum(in) - sum(out) for a day
-- equals that day's visitor_counts.count.
--
-- client_ref is a uuid minted per row -- the idempotency key kashi dedupes
-- on (branch_id, client_ref), exactly like a sales invoice's client_ref.
-- Rows are kept forever regardless of the feature flag; disabling the
-- counter only hides UI. Presses made while central sync was switched off
-- are logged here but never reach kashi (queueVisitorEventSync is a no-op
-- with no central_api_url), same "no retroactive backfill" behavior as
-- invoices.
CREATE TABLE IF NOT EXISTS visitor_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ref TEXT NOT NULL UNIQUE,
    day TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('in', 'out')),
    occurred_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_visitor_events_day ON visitor_events (day);
