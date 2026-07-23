create table if not exists clients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null,
    phone text not null unique,
    total_loyalty_points integer not null default 0,
    valid_loyalty_points integer not null default 0,
    created_at text not null default (datetime('now'))
);

create table if not exists coupons (
    code text primary key,
    status text not null check (status in ('active', 'inactive')),
    discount_type text not null check (discount_type in ('fixed', 'percentage')),
    reason text not null,
    client_id bigint not null references clients(id),
    valid_until text not null,
    created_at text not null default (datetime('now'))
);
