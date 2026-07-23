create table if not exists category_icons (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  icon text not null,
  categories text not null
);
