DELETE FROM feature_flags
WHERE key IN ('custom_item_prices', 'per_unit_item_prices');

ALTER TABLE invoice_products RENAME TO invoice_products_per_unit;

CREATE TABLE invoice_products (
    invoice_id bigint not null references invoices(id) on delete cascade,
    product_id bigint not null references products(id),
    unit_price bigint not null,
    line_total bigint not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    quantity bigint not null,
    primary key (invoice_id, product_id)
);

INSERT INTO invoice_products (
    invoice_id, product_id, unit_price, line_total, discount, quantity
)
SELECT
    invoice_id,
    product_id,
    MIN(unit_price),
    SUM(line_total),
    MIN(discount),
    SUM(quantity)
FROM invoice_products_per_unit
GROUP BY invoice_id, product_id;

DROP TABLE invoice_products_per_unit;
