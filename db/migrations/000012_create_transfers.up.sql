create table if not exists transfers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_inventory_id bigint not null references inventories(id),
    to_inventory_id bigint not null references inventories(id),
    type text not null check (type in ('assets', 'products')),
    created_at text not null default (datetime('now'))
);

create table if not exists transfer_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transfer_id bigint not null references transfers(id) on delete cascade,
    product_id bigint references products(id),
    asset_id bigint references assets(id),
    quantity bigint not null,
    check (
        (product_id is not null and asset_id is null) or
        (product_id is null and asset_id is not null)
    ),
    unique (transfer_id, product_id, asset_id)
);
