create table if not exists products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    barcode text not null,
    code text not null unique,
    name text not null,
    description text not null default '',
    is_active integer not null default 1,
    price bigint not null,
    discount integer not null check (discount >= 0 and discount <= 100),
    created_at text not null default (datetime('now'))
);

create table if not exists price_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null,
    product_id bigint references products(id),
    price bigint not null,
    created_at text not null default (datetime('now'))
);

create table if not exists discount_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null,
    product_id bigint references products(id),
    discount integer not null check (discount >= 0 and discount <= 100),
    created_at text not null default (datetime('now'))
);
