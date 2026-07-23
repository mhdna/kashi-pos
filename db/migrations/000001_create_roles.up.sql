create table if not exists roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique,
    description text not null default ''
);
