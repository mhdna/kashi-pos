-- name: GetDisplayPrefs :one
SELECT display_prefs, display_prefs_rev FROM settings LIMIT 1;

-- The screen's own edits: stored so they're reported to kashi, rev untouched.
-- name: SetLocalDisplayPrefs :exec
UPDATE settings SET display_prefs = ?;

-- kashi's push: stored and the rev bumped, so the screen adopts it.
-- name: SetRemoteDisplayPrefs :exec
UPDATE settings SET display_prefs = ?, display_prefs_rev = display_prefs_rev + 1;

