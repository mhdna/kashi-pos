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
-- kashi sends its whole set, so an event it no longer lists is one it deleted.
DELETE FROM calendar_events WHERE id NOT IN (sqlc.slice('ids'));
