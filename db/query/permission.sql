-- name: GetPermissionByID :one
SELECT *
FROM permissions
WHERE id = ?;

-- name: GetPermissionByName :one
SELECT *
FROM permissions
WHERE name = ?;

-- name: ListPermissions :many
SELECT *
FROM permissions
ORDER BY id;

-- name: CreatePermission :one
INSERT INTO permissions (
    name,
    description
)
VALUES (?, ?)
RETURNING *;

-- name: UpdatePermission :one
UPDATE permissions
SET name = ?,
    description = ?
WHERE id = ?
RETURNING *;

-- name: DeletePermission :exec
DELETE FROM permissions
WHERE id = ?;
