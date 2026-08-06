CREATE TABLE IF NOT EXISTS salesperson_attendance_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    salesperson_id INTEGER NOT NULL REFERENCES salespersons(id) ON DELETE CASCADE,
    attendance_user_id TEXT NOT NULL,
    salesperson_name TEXT NOT NULL,
    event_date TEXT NOT NULL,
    event_time TEXT NOT NULL,
    event_at TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    UNIQUE(device_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_salesperson_attendance_date
    ON salesperson_attendance_events(event_date, salesperson_id, event_at);

CREATE TABLE IF NOT EXISTS attendance_change_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    salesperson_id INTEGER NOT NULL REFERENCES salespersons(id) ON DELETE CASCADE,
    attendance_date TEXT NOT NULL,
    requested_time TEXT NOT NULL,
    note TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL,
    reviewed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_attendance_change_requests_date
    ON attendance_change_requests(attendance_date, salesperson_id, status);

INSERT OR IGNORE INTO feature_flags (key, enabled)
VALUES ('attendance_cashier_history', 0);
