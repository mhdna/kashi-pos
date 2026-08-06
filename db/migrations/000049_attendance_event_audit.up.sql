CREATE TABLE IF NOT EXISTS attendance_event_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id INTEGER NOT NULL REFERENCES attendance_change_requests(id),
    original_event_id INTEGER NOT NULL,
    device_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    salesperson_id INTEGER NOT NULL REFERENCES salespersons(id),
    attendance_user_id TEXT NOT NULL,
    salesperson_name TEXT NOT NULL,
    event_date TEXT NOT NULL,
    event_time TEXT NOT NULL,
    event_at TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    action TEXT NOT NULL,
    audited_at TEXT NOT NULL,
    approved_by TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attendance_event_audit_request ON attendance_event_audit(request_id);
CREATE INDEX IF NOT EXISTS idx_attendance_event_audit_date ON attendance_event_audit(event_date, salesperson_id);
