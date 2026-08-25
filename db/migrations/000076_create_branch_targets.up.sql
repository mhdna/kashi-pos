-- Read-only local cache of revenue targets kashi's admin sets for this
-- branch (see branch_targets in the kashi repo). Never written to locally
-- except by the sync-pull path in sync_pull.go -- remote_id is kashi's own
-- id, used to upsert rather than the natural-key matching used for
-- currencies/cashbox_accounts, since targets have no other stable key.
create table if not exists branch_targets (
    id integer primary key autoincrement,
    remote_id integer not null unique,
    date_from text not null,
    date_to text not null,
    target_amount_cents integer not null,
    color text not null default '',
    created_at text not null default (datetime('now'))
);
