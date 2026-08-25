-- Mirrors expenses (000074_create_expenses_table.up.sql): client_ref lives
-- directly on the row, generated at creation time and used as the sync
-- idempotency key kashi's /branch/loans endpoint dedupes on. category is
-- free text (the lender source, e.g. "Owner"/"External") -- kashi resolves
-- it to its own loan_categories row by name, this install never fetches or
-- caches that lookup table.
CREATE TABLE IF NOT EXISTS loans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ref text not null unique,
    description text not null,
    category text not null default '',
    amount bigint not null,
    currency_code text not null,
    cashbox_account_id bigint not null references cashbox_accounts(id),
    shift_id bigint references shifts(id),
    occurred_at text not null default (datetime('now', 'localtime')),
    created_at text not null default (datetime('now', 'localtime'))
);
