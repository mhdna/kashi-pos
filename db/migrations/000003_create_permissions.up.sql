create table if not exists permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique,
    description text not null default ''
);

create table if not exists role_permissions (
    role_id integer not null references roles(id) on delete cascade,
    permission_id integer not null references permissions(id) on delete cascade,
    primary key (role_id, permission_id)
);
