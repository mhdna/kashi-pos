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

-- name: ListAllVisitorCounts :many
-- Every day's tally, for the resync backfill of days counted before
-- per-press events were logged (see visitorCountBackfill).
SELECT day, count, updated_at FROM visitor_counts ORDER BY day;

-- name: ListVisitorCountsInRange :many
-- day is "YYYY-MM-DD" text, so a text comparison is a date comparison.
SELECT day, count FROM visitor_counts
WHERE day >= sqlc.arg(report_from) AND day <= sqlc.arg(report_to)
ORDER BY day;
