-- name: ListCentralQuickAccessProducts :many
-- kashi's branch list, resolved to this till's products by uid. A card for
-- a product this till doesn't have (yet) or that was deactivated is skipped,
-- same as ListQuickAccessProducts does for the till's own list.
SELECT sqlc.embed(p), c.color AS qa_color, c.text_color AS qa_text_color
FROM central_quick_access_items c
JOIN products p ON p.uid = c.product_uid
WHERE p.is_active = 1
ORDER BY c.position, c.product_uid;

-- name: ListCentralQuickAccessItems :many
SELECT * FROM central_quick_access_items ORDER BY position, product_uid;

-- name: DeleteCentralQuickAccessItems :exec
DELETE FROM central_quick_access_items;

-- name: InsertCentralQuickAccessItem :exec
INSERT INTO central_quick_access_items (product_uid, position, color, text_color)
VALUES (?, ?, ?, ?);

-- name: SetQuickAccessMode :exec
UPDATE settings SET quick_access_mode = ?, quick_access_mode_changed_at = ?;

-- name: SetQuickAccessReported :exec
UPDATE settings SET quick_access_reported = 1;

-- name: ListQuickAccessForReport :many
-- This till's own list, in card order, as kashi identifies products.
SELECT p.uid, q.color, q.text_color
FROM quick_access_items q
JOIN products p ON p.id = q.product_id
WHERE p.uid IS NOT NULL AND p.uid <> ''
ORDER BY q.position, q.created_at, q.product_id;
