CREATE TABLE price_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text NOT NULL UNIQUE,
    is_active integer NOT NULL DEFAULT 0,
    created_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE price_list_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    price_list_id bigint NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_code text NOT NULL,
    price bigint NOT NULL,
    created_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE(price_list_id, product_code)
);

CREATE TABLE discount_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text NOT NULL UNIQUE,
    is_active integer NOT NULL DEFAULT 0,
    created_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE discount_list_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    discount_list_id bigint NOT NULL REFERENCES discount_lists(id) ON DELETE CASCADE,
    product_code text NOT NULL,
    discount integer NOT NULL CHECK (discount >= 0 AND discount <= 100),
    created_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE(discount_list_id, product_code)
);

ALTER TABLE invoice_products ADD COLUMN price_list_id bigint REFERENCES price_lists(id);
ALTER TABLE invoice_products ADD COLUMN discount_list_id bigint REFERENCES discount_lists(id);
