CREATE TABLE IF NOT EXISTS client_account_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id bigint NOT NULL REFERENCES clients(id),
    invoice_id bigint NOT NULL UNIQUE REFERENCES invoices(id) ON DELETE CASCADE,
    amount bigint NOT NULL CHECK (amount > 0),
    created_at text NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS coupon_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text NOT NULL UNIQUE,
    value_cents bigint NOT NULL CHECK (value_cents > 0),
    currency_code text NOT NULL DEFAULT 'USD',
    is_active integer NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS invoice_coupon_payments (
    invoice_id bigint PRIMARY KEY REFERENCES invoices(id) ON DELETE CASCADE,
    coupon_code text NOT NULL REFERENCES coupons(code),
    coupon_type_id bigint NOT NULL REFERENCES coupon_types(id),
    amount bigint NOT NULL CHECK (amount > 0)
);

ALTER TABLE coupons ADD COLUMN type_id bigint REFERENCES coupon_types(id);
ALTER TABLE coupons ADD COLUMN value_cents bigint;
ALTER TABLE coupons ADD COLUMN redeemed_invoice_id bigint REFERENCES invoices(id);

CREATE INDEX IF NOT EXISTS idx_client_account_entries_client ON client_account_entries(client_id, created_at);
CREATE INDEX IF NOT EXISTS idx_coupons_type ON coupons(type_id);
