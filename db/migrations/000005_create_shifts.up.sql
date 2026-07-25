create table if not exists cashbox_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique,
    currency_code text not null default 'USD'
);

create table if not exists shifts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    is_closed integer not null default 0,
    created_at text not null default (datetime('now', 'localtime')),
    closed_at text not null default ''
);

create table if not exists accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique
);

create table if not exists shifts_accounts_balances (
    account_id bigint not null references cashbox_accounts(id),
    shift_id bigint not null references shifts(id),
    balance bigint not null,
    primary key (account_id, shift_id)
);
