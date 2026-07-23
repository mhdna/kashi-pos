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
