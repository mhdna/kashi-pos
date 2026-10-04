-- Price lists are a read-only mirror of kashi's since 000097. There are no
-- create/update/activate queries any more: the only writer is the sync, and
-- it writes a whole list at a time, because kashi sends a list complete with
-- its items.

-- name: ListPriceLists :many
SELECT * FROM price_lists ORDER BY name;

-- name: GetPriceListByID :one
SELECT * FROM price_lists WHERE id = ?;

-- name: GetPriceListByUID :one
SELECT * FROM price_lists WHERE uid = ?;

-- name: ListPriceListItems :many
SELECT * FROM price_list_items WHERE price_list_id = ? ORDER BY product_uid;

-- name: UpsertPriceListByUID :one
-- Matched by uid, never by name or id -- a till's ids are its own, and a list
-- renamed centrally must stay the same list here.
INSERT INTO price_lists (uid, name, is_active, is_default, is_special)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (uid) DO UPDATE SET
    name = excluded.name,
    is_active = excluded.is_active,
    is_default = excluded.is_default,
    is_special = excluded.is_special
RETURNING *;

-- name: DeletePriceListItemsForList :exec
-- Half of the replace-on-apply: kashi has no way to tell a till "this one
-- item is gone", so an applied list is cleared and refilled and a removal is
-- expressed by the item simply not being in the payload.
DELETE FROM price_list_items WHERE price_list_id = ?;

-- name: InsertPriceListItem :exec
INSERT INTO price_list_items (price_list_id, product_uid, price)
VALUES (?, ?, ?)
ON CONFLICT (price_list_id, product_uid) DO UPDATE SET price = excluded.price;

-- name: DeletePriceListByUID :exec
DELETE FROM price_lists WHERE uid = ?;

-- name: ListActivePriceListItems :many
-- Every price in force: the default list, plus this branch's own list if
-- kashi assigned it one.
--
-- Ordered is_default DESC so the default list's row for a product is read
-- first and a branch list's row for the same product overwrites it --
-- loadActiveOverrides builds a map keyed by product uid, so last write wins.
-- A product only the default list mentions keeps the default price.
SELECT pli.product_uid, pli.price, pl.id AS list_id, pl.name AS list_name, pl.is_special
FROM price_list_items pli
JOIN price_lists pl ON pl.id = pli.price_list_id
WHERE pl.is_active = 1
ORDER BY pli.product_uid, pl.is_default DESC;
