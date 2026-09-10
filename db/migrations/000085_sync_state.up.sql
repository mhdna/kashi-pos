-- Adds the bookkeeping the sync status UI and the "sync everything" flow
-- need: which central server a queued row was actually accepted by, and a
-- one-row scratchpad for the last full reconcile.
--
-- synced_to lets a per-invoice dot mean "kashi confirmed it, on the server
-- we're pointed at now" -- re-pointing central_api_url at a different host
-- leaves every historical 'synced' row stamped with the old URL, so the UI
-- correctly drops back to "not confirmed here" until a reconcile re-checks.
ALTER TABLE sync_outbox ADD COLUMN synced_to text NOT NULL DEFAULT '';

-- Permanently-rejected rows (a 4xx from kashi, or an entity_type kashi has
-- no route for) move to 'failed' instead of being retried forever and
-- clogging the 20-row drain batch. status now goes pending -> synced or
-- pending -> failed; a failed row is only ever retried by an explicit
-- resync.
CREATE TABLE IF NOT EXISTS sync_state (
    id integer primary key check (id = 1),
    current_target text not null default '',
    last_reconcile_at text,
    last_reconcile_json text not null default ''
);
INSERT OR IGNORE INTO sync_state (id) VALUES (1);
