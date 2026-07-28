CREATE TABLE IF NOT EXISTS attendance_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    akuvox_ip TEXT NOT NULL DEFAULT '',
    akuvox_username TEXT NOT NULL DEFAULT '',
    akuvox_password TEXT NOT NULL DEFAULT '',
    attendance_enabled INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO attendance_config (id) VALUES (1);

CREATE TABLE IF NOT EXISTS attendance_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    employee_id INTEGER NOT NULL REFERENCES users(id),
    akuvox_user_id TEXT NOT NULL,
    employee_name TEXT NOT NULL,
    event_date TEXT NOT NULL,
    event_time TEXT NOT NULL,
    event_at TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    UNIQUE(device_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_attendance_events_date ON attendance_events(event_date, employee_id, event_at);

CREATE TABLE IF NOT EXISTS attendance_complaints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_id INTEGER NOT NULL REFERENCES users(id),
    attendance_date TEXT NOT NULL,
    requested_time TEXT NOT NULL,
    note TEXT NOT NULL,
    approved INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attendance_complaints_date ON attendance_complaints(attendance_date, employee_id);
