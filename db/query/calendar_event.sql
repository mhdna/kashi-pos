-- name: ListCalendarEvents :many
SELECT * FROM calendar_events ORDER BY start_date, id;

-- name: UpsertCalendarEvent :exec
INSERT INTO calendar_events (id, name, icon, color, start_date, end_date)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    icon = excluded.icon,
    color = excluded.color,
    start_date = excluded.start_date,
    end_date = excluded.end_date;

-- name: DeleteCalendarEventsNotIn :exec
-- Central sends its whole set, so an event it no longer lists is one it deleted.
DELETE FROM calendar_events WHERE id NOT IN (sqlc.slice('ids'));

-- name: GetCalendarSettings :one
SELECT * FROM calendar_settings WHERE id = 1;

-- name: SetCalendarSettings :exec
UPDATE calendar_settings SET business_type = ?, hidden_events = ?, catalog = ? WHERE id = 1;
