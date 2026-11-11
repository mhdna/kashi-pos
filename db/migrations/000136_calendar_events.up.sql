-- User-defined calendar events, mirrored whole from central (sync entity
-- "calendar_events") for the Events page. id is central's own id. Dates are
-- YYYY-MM-DD text; the till never edits these rows.
CREATE TABLE IF NOT EXISTS calendar_events (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT 'mdi-calendar-star',
    color TEXT NOT NULL DEFAULT 'primary',
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL
);
