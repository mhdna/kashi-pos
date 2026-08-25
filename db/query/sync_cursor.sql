-- name: GetSyncCursor :one
SELECT cursor FROM sync_cursors WHERE entity = ?;

-- name: SetSyncCursor :exec
INSERT INTO sync_cursors (entity, cursor) VALUES (?, ?)
ON CONFLICT (entity) DO UPDATE SET cursor = excluded.cursor;
