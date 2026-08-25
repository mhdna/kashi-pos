-- Queues invoices for push to the central kashi backend (see
-- POST /branch/sales_invoices and /branch/return_invoices in the kashi
-- repo). payload_json is a frozen snapshot of exactly what gets POSTed,
-- captured at invoice-creation time — not re-derived from other tables on
-- each retry — so a later schema change here can never alter what an
-- already-queued invoice reports.
create table if not exists sync_outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type text not null,
    client_ref text not null unique,
    payload_json text not null,
    status text not null default 'pending',
    attempt_count integer not null default 0,
    last_error text not null default '',
    next_attempt_at text not null default (datetime('now')),
    created_at text not null default (datetime('now')),
    synced_at text
);

create index if not exists idx_sync_outbox_status_next_attempt
    on sync_outbox (status, next_attempt_at);
