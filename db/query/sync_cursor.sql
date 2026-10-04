-- name: GetSyncCursor :one
SELECT cursor FROM sync_cursors WHERE entity = ?;

-- name: SetSyncCursor :exec
INSERT INTO sync_cursors (entity, cursor) VALUES (?, ?)
ON CONFLICT (entity) DO UPDATE SET cursor = excluded.cursor;

-- name: ClearSyncCursorsExcept :exec
-- "Sync everything" against the server this till already syncs with:
-- re-pull the small entities from scratch, but keep the big ones' cursors
-- (clients, products) -- the completeness check covers those by comparing
-- ids and versions instead of re-downloading the whole catalog.
DELETE FROM sync_cursors WHERE entity NOT IN (sqlc.slice('keep'));
