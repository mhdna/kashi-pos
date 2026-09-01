-- name: GetVisitorCount :one
SELECT count FROM visitor_counts WHERE day = ?;

-- name: SetVisitorCount :exec
INSERT INTO visitor_counts (day, count, updated_at)
VALUES (sqlc.arg(day), sqlc.arg(count), datetime('now', 'localtime'))
ON CONFLICT(day) DO UPDATE SET
    count = excluded.count,
    updated_at = excluded.updated_at;

-- name: ListVisitorCountsForMonth :many
SELECT day, count FROM visitor_counts
WHERE day LIKE sqlc.arg(month_prefix)
ORDER BY day;
