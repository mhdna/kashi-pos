-- name: GetSyncState :one
SELECT * FROM sync_state WHERE id = 1;

-- name: SetSyncTarget :exec
UPDATE sync_state SET current_target = ? WHERE id = 1;

-- name: SetLastReconcile :exec
UPDATE sync_state
SET last_reconcile_at = datetime('now'), last_reconcile_json = ?
WHERE id = 1;
