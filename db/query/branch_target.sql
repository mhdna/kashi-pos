-- name: UpsertBranchTarget :exec
INSERT INTO branch_targets (remote_id, date_from, date_to, target_amount_cents, color)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (remote_id) DO UPDATE SET
    date_from = excluded.date_from,
    date_to = excluded.date_to,
    target_amount_cents = excluded.target_amount_cents,
    color = excluded.color;

-- name: ListBranchTargets :many
SELECT * FROM branch_targets ORDER BY target_amount_cents ASC;

-- name: ListBranchTargetsOverlapping :many
-- Targets whose [date_from, date_to] range intersects [range_from,
-- range_to]. kashi-pos only ever shows the current calendar month, so
-- callers pass this month's first and last day (both "YYYY-MM-DD", which
-- sorts correctly as text). A quarter- or year-long target set centrally
-- still shows while the current month falls inside it.
SELECT * FROM branch_targets
WHERE date_to >= sqlc.arg(range_from) AND date_from <= sqlc.arg(range_to)
ORDER BY target_amount_cents ASC;

-- name: DeleteBranchTargetByRemoteID :exec
-- Fires when kashi reports a target as no longer active (see
-- applySyncedBranchTarget in sync_pull.go) -- this is just a read-only
-- mirror of kashi's roster, so removing the local row outright is fine,
-- unlike synced entities with local-only fields to preserve.
DELETE FROM branch_targets WHERE remote_id = ?;

-- name: DeleteBranchTargetProgress :exec
DELETE FROM branch_target_progress;

-- name: InsertBranchTargetProgress :exec
INSERT INTO branch_target_progress (target_id, achieved_cents, fetched_at)
VALUES (?, ?, ?);

-- name: GetBranchTargetProgress :one
SELECT * FROM branch_target_progress WHERE target_id = ?;
