-- Read-only mirror of kashi's discount lists -- see pricelist.sql, which this
-- file mirrors exactly.

-- name: ListDiscountLists :many
SELECT * FROM discount_lists ORDER BY name;

-- name: GetDiscountListByID :one
SELECT * FROM discount_lists WHERE id = ?;

-- name: GetDiscountListByUID :one
SELECT * FROM discount_lists WHERE uid = ?;

-- name: ListDiscountListItems :many
SELECT * FROM discount_list_items WHERE discount_list_id = ? ORDER BY product_uid;

-- name: UpsertDiscountListByUID :one
INSERT INTO discount_lists (uid, name, is_active, is_default, is_special)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (uid) DO UPDATE SET
    name = excluded.name,
    is_active = excluded.is_active,
    is_default = excluded.is_default,
    is_special = excluded.is_special
RETURNING *;

-- name: DeleteDiscountListItemsForList :exec
DELETE FROM discount_list_items WHERE discount_list_id = ?;

-- name: InsertDiscountListItem :exec
INSERT INTO discount_list_items (discount_list_id, product_uid, discount, deal_uid)
VALUES (?, ?, ?, ?)
ON CONFLICT (discount_list_id, product_uid) DO UPDATE SET discount = excluded.discount, deal_uid = excluded.deal_uid;

-- name: DeleteDiscountListByUID :exec
DELETE FROM discount_lists WHERE uid = ?;

-- name: ListActiveDiscountListItems :many
-- See ListActivePriceListItems for why this is ordered by is_default DESC.
SELECT dli.product_uid, dli.discount, dli.deal_uid, dl.id AS list_id, dl.name AS list_name, dl.is_special
FROM discount_list_items dli
JOIN discount_lists dl ON dl.id = dli.discount_list_id
WHERE dl.is_active = 1
ORDER BY dli.product_uid, dl.is_default DESC;

-- name: UpsertDeal :exec
-- Deals arrive with the lists naming them (sync_pull_lists.go); a deal two
-- lists share is the same row.
INSERT INTO deals (uid, name, kind, buy_qty, get_qty, percent, bundle_price, fallback_percent, fallback_only_without_set)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (uid) DO UPDATE SET
    name = excluded.name, kind = excluded.kind, buy_qty = excluded.buy_qty,
    get_qty = excluded.get_qty, percent = excluded.percent, bundle_price = excluded.bundle_price,
    fallback_percent = excluded.fallback_percent,
    fallback_only_without_set = excluded.fallback_only_without_set;

-- name: ListDeals :many
SELECT * FROM deals ORDER BY uid;
