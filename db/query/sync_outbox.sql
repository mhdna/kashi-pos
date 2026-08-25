-- name: CreateSyncOutboxEntry :one
INSERT INTO sync_outbox (entity_type, client_ref, payload_json)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListDueSyncOutboxEntries :many
SELECT * FROM sync_outbox
WHERE status = 'pending' AND next_attempt_at <= datetime('now')
ORDER BY id
LIMIT ?;

-- name: MarkSyncOutboxSynced :exec
UPDATE sync_outbox
SET status = 'synced', synced_at = datetime('now')
WHERE id = ?;

-- name: MarkSyncOutboxAttemptFailed :exec
UPDATE sync_outbox
SET attempt_count = attempt_count + 1, last_error = ?, next_attempt_at = ?
WHERE id = ?;

-- name: CountPendingSyncOutbox :one
SELECT COUNT(*) FROM sync_outbox WHERE status = 'pending';
