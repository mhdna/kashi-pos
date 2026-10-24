-- Sale options, mirrored from kashi (its 000199): a product can make the
-- till ask which option is being sold -- Summer or Winter for outlet stock
-- whose one barcode covers both -- and a discount list can hold a discount
-- that applies only when the variant is sold as that option. Options are
-- known here by name only, like colors and attributes.

-- Which options a product asks for, in the order the till shows them.
CREATE TABLE product_sale_options (
    product_id integer not null references products(id) on delete cascade,
    name text not null,
    sort_order integer not null default 0,
    primary key (product_id, name)
);

-- Kept apart from discount_list_items for the same reason kashi does: every
-- existing reader of discount_list_items keeps meaning the plain discount.
CREATE TABLE discount_list_option_items (
    id integer primary key autoincrement,
    discount_list_id bigint not null references discount_lists(id) on delete cascade,
    product_uid text not null,
    sale_option text not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    unique (discount_list_id, product_uid, sale_option)
);
CREATE INDEX idx_discount_list_option_items_product_uid ON discount_list_option_items(product_uid);

-- What the line was sold as; '' for a product with no options. (A held cart
-- keeps its lines' options in hold_invoices.snapshot_json.)
ALTER TABLE invoice_products ADD COLUMN sale_option text not null default '';
