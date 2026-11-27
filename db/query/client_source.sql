-- name: ListClientSources :many
SELECT * FROM client_sources ORDER BY sort_order, uid;

-- name: ListActiveClientSources :many
SELECT * FROM client_sources WHERE is_active = 1 ORDER BY sort_order, uid;

-- name: UpsertClientSource :exec
INSERT INTO client_sources (uid, name, sort_order, is_active) VALUES (?, ?, ?, ?)
ON CONFLICT(uid) DO UPDATE SET name = excluded.name, sort_order = excluded.sort_order, is_active = excluded.is_active;

-- name: DeleteClientSource :exec
DELETE FROM client_sources WHERE uid = ?;

-- name: ClientSourceExists :one
SELECT COUNT(*) FROM client_sources WHERE uid = ?;

-- name: SetClientSourceUID :exec
UPDATE clients SET source_uid = ? WHERE id = ?;
