-- Local mirrors of kashi's branch-scoped expense_categories /
-- loan_categories, kept in step by sync_pull.go (see applySyncedCurrency
-- for the create-or-update-by-natural-key pattern). Only scope='branch'
-- rows are ever pushed down, so there is no scope column here. icon holds
-- an mdi glyph name, color a Vuetify colour token -- rendered the same way
-- in kashi and here so a category looks identical in both apps.
--
-- expenses.category / loans.category stay as plain name text: the category
-- picker writes the selected category's *name*, and kashi's /branch
-- endpoints still resolve that name to their own category_id via
-- get-or-create. These tables only back the dropdown.
CREATE TABLE IF NOT EXISTS expense_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique,
    is_active integer not null default 1,
    icon text not null default 'mdi-tag-outline',
    color text not null default 'blue-grey',
    updated_at text not null default (datetime('now'))
);

CREATE TABLE IF NOT EXISTS loan_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name text not null unique,
    is_active integer not null default 1,
    icon text not null default 'mdi-tag-outline',
    color text not null default 'blue-grey',
    updated_at text not null default (datetime('now'))
);
