-- Each row is a sellable product variant. Product codes may repeat; barcodes
-- identify the exact variant. Existing product IDs are preserved so invoices,
-- inventory, and hold invoices remain valid.
PRAGMA foreign_keys = OFF;

CREATE TABLE products_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    barcode text NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active integer NOT NULL DEFAULT 1,
    price bigint NOT NULL,
    discount integer NOT NULL DEFAULT 0 CHECK (discount >= 0 AND discount <= 100),
    color text NOT NULL DEFAULT '',
    size text NOT NULL DEFAULT '',
    currency_code text NOT NULL DEFAULT 'USD',
    created_at text NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO products_new (id, barcode, code, name, description, is_active, price, discount, created_at)
SELECT id, barcode, code, name, description, is_active, price, discount, created_at
FROM products;

DROP TABLE products;
ALTER TABLE products_new RENAME TO products;

CREATE UNIQUE INDEX idx_products_barcode ON products(barcode);
CREATE INDEX idx_products_code ON products(code);
