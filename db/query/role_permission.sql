-- name: GetRolePermissionsByRoleID :many
SELECT p.*
FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = ?;

-- name: AssignPermissionToRole :exec
INSERT INTO role_permissions (
    role_id,
    permission_id
)
VALUES (?, ?);

-- name: RemovePermissionFromRole :exec
DELETE FROM role_permissions
WHERE role_id = ? AND permission_id = ?;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions
WHERE role_id = ?;
