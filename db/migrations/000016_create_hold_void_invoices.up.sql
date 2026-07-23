create table if not exists hold_invoices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  items_json text not null,
  created_at text not null default (datetime('now'))
);

create table if not exists void_invoices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  items_json text not null,
  reason text not null default '',
  created_at text not null default (datetime('now'))
);
