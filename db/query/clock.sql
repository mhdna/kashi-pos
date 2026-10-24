-- name: SetSettingsTimezone :exec
UPDATE settings SET timezone = ?;
