-- name: GetRoleByID :one
SELECT *
FROM roles
WHERE id = ?;

-- name: GetRoleByName :one
SELECT *
FROM roles
WHERE name = ?;

-- name: ListRoles :many
SELECT *
FROM roles
ORDER BY id;

-- name: CreateRole :one
INSERT INTO roles (
    name,
    description
)
VALUES (?, ?)
RETURNING *;

-- name: UpdateRole :one
UPDATE roles
SET name = ?,
    description = ?
WHERE id = ?
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles
WHERE id = ?;
