-- name: GetSalespersonByID :one
SELECT * FROM salespersons WHERE id = ?;

-- name: ListSalespersons :many
SELECT * FROM salespersons ORDER BY id;

-- name: CreateSalesperson :one
INSERT INTO salespersons (full_name) VALUES (?) RETURNING *;

-- name: UpdateSalesperson :one
UPDATE salespersons SET full_name = ? WHERE id = ? RETURNING *;

-- name: DeleteSalesperson :exec
DELETE FROM salespersons WHERE id = ?;

-- name: GetSalespersonByRemoteID :one
SELECT * FROM salespersons WHERE remote_id = ?;

-- name: UpdateSalespersonAttendanceID :exec
UPDATE salespersons SET attendance_user_id = ? WHERE id = ?;

-- name: CreateSyncedSalesperson :one
INSERT INTO salespersons (full_name, remote_id, is_active) VALUES (?, ?, ?) RETURNING *;

-- name: UpdateSyncedSalesperson :one
-- attendance_user_id is deliberately not touched here -- it's local-only
-- device pairing, never sent by kashi (see sync_pull.go).
UPDATE salespersons SET full_name = ?, is_active = ? WHERE remote_id = ? RETURNING *;
