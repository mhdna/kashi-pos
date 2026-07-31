ALTER TABLE invoice_products RENAME TO invoice_products_price_and_discount;

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
SELECT
    invoice_id,
    product_id,
    unit_price,
    SUM(line_total),
    MIN(discount),
    SUM(quantity)
FROM invoice_products_price_and_discount
GROUP BY invoice_id, product_id, unit_price;

DROP TABLE invoice_products_price_and_discount;
