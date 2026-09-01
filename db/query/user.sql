-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = ?;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = ?;

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY id;

-- name: CreateUser :one
INSERT INTO users (
    username,
    password_hash,
    role_id,
    active
)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET username = ?,
    password_hash = ?,
    role_id = ?,
    active = ?
WHERE id = ?
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: GetUserByRemoteID :one
SELECT * FROM users WHERE remote_id = ?;

-- name: CreateSyncedUser :one
INSERT INTO users (username, password_hash, role_id, active, attendance_user_id, remote_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: AdoptUserAsSynced :one
-- A user the till already created locally (e.g. a set_branch_user_pin
-- command landed before the first roster pull) is matched by username and
-- linked to its kashi row. password_hash is left as-is.
UPDATE users
SET role_id = ?, active = ?, attendance_user_id = ?, remote_id = ?
WHERE username = ?
RETURNING *;

-- name: UpdateSyncedUser :one
-- password_hash is deliberately not touched -- set only via
-- set_branch_user_pin or a local admin reset.
UPDATE users
SET username = ?, role_id = ?, active = ?, attendance_user_id = ?
WHERE remote_id = ?
RETURNING *;

-- name: SetUserPasswordHashByUsername :execrows
UPDATE users SET password_hash = ? WHERE username = ?;
