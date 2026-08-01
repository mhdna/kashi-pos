CREATE TABLE IF NOT EXISTS client_account_payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id bigint NOT NULL REFERENCES client_account_entries(invoice_id) ON DELETE CASCADE,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    shift_id bigint NOT NULL REFERENCES shifts(id),
    amount bigint NOT NULL CHECK (amount > 0),
    created_at text NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_client_account_payments_invoice
    ON client_account_payments(invoice_id, created_at);
CREATE INDEX IF NOT EXISTS idx_client_account_payments_shift
    ON client_account_payments(shift_id, cashbox_id);
