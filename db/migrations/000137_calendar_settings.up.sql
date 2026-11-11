-- Which built-in calendar central uses and which of its events are switched off,
-- mirrored from central (sync entity "calendar_settings"). One row; the till
-- never edits it. hidden_events is a JSON array of event names.
CREATE TABLE IF NOT EXISTS calendar_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    business_type TEXT NOT NULL DEFAULT 'clothing',
    hidden_events TEXT NOT NULL DEFAULT '[]'
);
INSERT OR IGNORE INTO calendar_settings (id) VALUES (1);
