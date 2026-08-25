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
