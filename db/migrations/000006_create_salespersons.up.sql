create table if not exists salespersons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name text not null,
    attendance_user_id text not null default ''
);
