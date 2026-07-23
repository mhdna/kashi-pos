create table if not exists settings (
    branch_name text not null,
    api_key text not null,
    printer_id text not null,
    printer_size text not null,
    receipt_printer text not null default '',
    screen_port text not null default '',
    receipt_width integer not null default 80,
    receipt_height integer not null default 297,
    theme text not null default 'system'
);

create table if not exists payment_methods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
	name text not null,
	value_in_usd not null
);
