CREATE TABLE IF NOT EXISTS shift_balance_replacements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    shift_id INTEGER NOT NULL REFERENCES shifts(id),
    from_cashbox_id INTEGER NOT NULL REFERENCES cashbox_accounts(id),
    to_cashbox_id INTEGER NOT NULL REFERENCES cashbox_accounts(id),
    amount INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_shift_balance_replacements_shift
    ON shift_balance_replacements(shift_id);
