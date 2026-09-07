-- name: CreateVisitorEvent :exec
INSERT INTO visitor_events (client_ref, day, direction, occurred_at)
VALUES (sqlc.arg(client_ref), sqlc.arg(day), sqlc.arg(direction), sqlc.arg(occurred_at));

-- name: ListVisitorEventsForDay :many
SELECT client_ref, day, direction, occurred_at
FROM visitor_events
WHERE day = sqlc.arg(day)
ORDER BY occurred_at, id;
