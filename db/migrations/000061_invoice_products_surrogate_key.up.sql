-- invoice_products previously used (invoice_id, product_id, unit_price,
-- discount) as its primary key. That collides whenever an invoice needs two
-- distinct lines for the same product at the same price/discount - notably
-- an exchange returning a product and replacing it with a fresh unit of the
-- same product at the same price, which now legitimately produces one
-- negative-quantity line and one positive-quantity line that would
-- otherwise share the same key.
ALTER TABLE invoice_products RENAME TO invoice_products_composite_key;

CREATE TABLE invoice_products (
    id integer primary key autoincrement,
    invoice_id bigint not null references invoices(id) on delete cascade,
    product_id bigint not null references products(id),
    unit_price bigint not null,
    line_total bigint not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    quantity bigint not null,
    salesperson_id bigint not null default 0,
    note text not null default '',
    price_list_id bigint references price_lists(id),
    discount_list_id bigint references discount_lists(id)
);

INSERT INTO invoice_products (
    invoice_id, product_id, unit_price, line_total, discount, quantity,
    salesperson_id, note, price_list_id, discount_list_id
)
SELECT
    invoice_id, product_id, unit_price, line_total, discount, quantity,
    salesperson_id, note, price_list_id, discount_list_id
FROM invoice_products_composite_key;

DROP TABLE invoice_products_composite_key;

CREATE INDEX idx_invoice_products_invoice_id ON invoice_products(invoice_id);
CREATE INDEX idx_invoice_products_invoice_product ON invoice_products(invoice_id, product_id);
