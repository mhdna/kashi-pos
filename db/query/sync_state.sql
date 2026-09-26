-- name: GetSyncState :one
SELECT * FROM sync_state WHERE id = 1;

-- name: SetSyncTarget :exec
UPDATE sync_state SET current_target = ? WHERE id = 1;

-- name: SetLastReconcile :exec
UPDATE sync_state
SET last_reconcile_at = datetime('now'), last_reconcile_json = ?
WHERE id = 1;

-- name: SetIdentityAdoptedAt :exec
UPDATE sync_state SET identity_adopted_at = ? WHERE id = 1;

-- name: SetLastFullCheck :exec
UPDATE sync_state SET last_full_check_at = ?, last_full_check_json = ? WHERE id = 1;
