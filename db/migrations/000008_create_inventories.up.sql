create table if not exists inventories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique
);

create table if not exists inventory_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    inventory_id bigint not null references inventories(id),
    product_id bigint not null references products(id),
    quantity bigint not null,
    unique (inventory_id, product_id)
);
