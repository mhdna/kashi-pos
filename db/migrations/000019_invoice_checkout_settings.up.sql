ALTER TABLE settings ADD COLUMN rounding_mode text NOT NULL DEFAULT 'half_up';
ALTER TABLE settings ADD COLUMN receipt_enabled integer NOT NULL DEFAULT 0 CHECK (receipt_enabled IN (0, 1));
ALTER TABLE settings ADD COLUMN sales_inventory_id bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS invoice_payments (
    invoice_id bigint NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    amount bigint NOT NULL CHECK (amount > 0),
    PRIMARY KEY (invoice_id, cashbox_id)
);

CREATE TABLE IF NOT EXISTS invoice_payment_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id bigint NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    amount bigint NOT NULL CHECK (amount <> 0),
    entry_type text NOT NULL CHECK (entry_type IN ('payment', 'change')),
    created_at text NOT NULL DEFAULT (datetime('now'))
);
