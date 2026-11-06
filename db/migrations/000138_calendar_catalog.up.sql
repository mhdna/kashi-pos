-- kashi's built-in calendar events as JSON date rules (calendar_settings sync
-- entity), so the Events page follows kashi offline. '' = not received yet; the
-- page then uses the copy bundled in the app.
ALTER TABLE calendar_settings ADD COLUMN catalog TEXT NOT NULL DEFAULT '';
