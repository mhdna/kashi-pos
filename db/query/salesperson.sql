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
INSERT INTO salespersons (full_name, remote_id, is_active, color) VALUES (?, ?, ?, ?) RETURNING *;

-- name: AdoptSalespersonByName :one
-- First roster pull on a till that already had its own salespersons: link
-- the till's unlinked salesperson with the same name to kashi's record
-- instead of creating a duplicate. Keeps its local id, so invoices and
-- attendance pairing stay attached.
UPDATE salespersons
SET remote_id = sqlc.arg(remote_id),
    full_name = sqlc.arg(full_name),
    is_active = sqlc.arg(is_active),
    color = sqlc.arg(color)
WHERE id = (
    SELECT id FROM salespersons
    WHERE remote_id IS NULL
      AND lower(trim(full_name)) = lower(trim(sqlc.arg(full_name)))
    ORDER BY id
    LIMIT 1
)
RETURNING *;

-- name: UpdateSyncedSalesperson :one
-- attendance_user_id is deliberately not touched here -- it's local-only
-- device pairing, never sent by kashi (see sync_pull.go).
UPDATE salespersons SET full_name = ?, is_active = ?, color = ? WHERE remote_id = ? RETURNING *;

-- name: SetSalespersonAttendanceIDByRemoteID :exec
-- kashi decided this salesperson's device number (attendance_user_id_set in
-- the salesperson feed); it replaces whatever was typed in here.
UPDATE salespersons SET attendance_user_id = ? WHERE remote_id = ?;

-- name: SetSalespersonIsSalespersonByRemoteID :exec
UPDATE salespersons SET is_salesperson = ? WHERE remote_id = ?;
