create table if not exists users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username text not null unique,
    password_hash text not null,
    role_id integer not null references roles(id),
    active integer not null default 1,
    attendance_user_id text not null default ''
);
