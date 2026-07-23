create table if not exists hold_invoices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at text not null default (datetime('now'))
);

create table if not exists hold_invoice_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  hold_invoice_id bigint not null references hold_invoices(id) on delete cascade,
  product_id bigint not null references products(id),
  qty integer not null
);

create table if not exists void_invoices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  items_json text not null,
  reason text not null default '',
  created_at text not null default (datetime('now'))
);
