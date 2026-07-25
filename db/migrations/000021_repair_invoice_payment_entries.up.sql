-- Some databases were marked as upgraded without receiving the payment-entry
-- table. Keep this migration idempotent so it repairs those databases without
-- affecting installations where migration 19 already created the table.
CREATE TABLE IF NOT EXISTS invoice_payment_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id bigint NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    amount bigint NOT NULL CHECK (amount <> 0),
    entry_type text NOT NULL CHECK (entry_type IN ('payment', 'change')),
    created_at text NOT NULL DEFAULT (datetime('now'))
);
