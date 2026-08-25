-- client_ref lives directly on the row (unlike invoices, which needed the
-- separate invoice_client_refs table because the invoices table predates
-- sync) — generated at creation time and used as the sync idempotency key
-- kashi's /branch/expenses endpoint dedupes on.
CREATE TABLE IF NOT EXISTS expenses (
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
