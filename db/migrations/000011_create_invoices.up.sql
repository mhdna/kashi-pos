create table if not exists invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cashbox_id bigint not null references cashbox_accounts(id),
    shift_id bigint not null references shifts(id),
    invoice_code text not null,
    invoice_index bigint not null,
    year integer not null,
    client_id bigint not null references clients(id),
    inventory_id bigint not null references inventories(id),
    discount integer not null check (discount >= 0 and discount <= 100),
    subtotal bigint not null,
    discounted_total bigint not null,
    grand_total bigint not null,
    created_at text not null default (datetime('now'))
);

create table if not exists invoice_products (
    invoice_id bigint not null references invoices(id) on delete cascade,
    product_id bigint not null references products(id),
    unit_price bigint not null,
    line_total bigint not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    quantity bigint not null,
    primary key (invoice_id, product_id)
);

create table if not exists invoice_indexes (
    year integer not null,
    cashbox_id bigint not null references cashbox_accounts(id),
    last_index bigint not null,
    type text not null check (type in ('sales', 'return')),
    primary key (cashbox_id, year)
);

create table if not exists sales_invoices (
    invoice_id bigint primary key references invoices(id) on delete cascade
);

create table if not exists return_invoices (
    invoice_id bigint primary key references invoices(id) on delete cascade,
    sales_invoice_id bigint not null references sales_invoices(invoice_id)
);
