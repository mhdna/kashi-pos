CREATE TABLE quick_access_items (
    product_id bigint PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    position integer NOT NULL DEFAULT 0,
    created_at text NOT NULL DEFAULT (datetime('now'))
);
