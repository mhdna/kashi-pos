-- name: GetSettings :one
SELECT * FROM settings LIMIT 1;

-- name: UpdateSettings :exec
UPDATE settings
SET branch_name = ?,
    api_key = ?,
    printer_id = ?,
    printer_size = ?,
    theme = ?;

-- name: UpdateTheme :exec
UPDATE settings SET theme = ?;

-- name: UpsertSettings :exec
INSERT INTO settings (branch_name, api_key, printer_id, printer_size, theme)
VALUES (?, ?, ?, ?, ?);
