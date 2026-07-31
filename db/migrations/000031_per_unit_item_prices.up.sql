INSERT OR IGNORE INTO feature_flags (key, enabled)
SELECT 'custom_item_prices', enabled
FROM feature_flags
WHERE key = 'custom_item_discounts';

INSERT OR IGNORE INTO feature_flags (key, enabled)
VALUES ('per_unit_item_prices', 0);

ALTER TABLE invoice_products RENAME TO invoice_products_legacy;

CREATE TABLE invoice_products (
    invoice_id bigint not null references invoices(id) on delete cascade,
    product_id bigint not null references products(id),
    unit_price bigint not null,
    line_total bigint not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    quantity bigint not null,
    primary key (invoice_id, product_id, unit_price)
);

INSERT INTO invoice_products (
    invoice_id, product_id, unit_price, line_total, discount, quantity
)
SELECT invoice_id, product_id, unit_price, line_total, discount, quantity
FROM invoice_products_legacy;

DROP TABLE invoice_products_legacy;
