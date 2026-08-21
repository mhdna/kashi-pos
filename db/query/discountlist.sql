-- name: ListDiscountLists :many
SELECT * FROM discount_lists ORDER BY name;

-- name: GetDiscountListByID :one
SELECT * FROM discount_lists WHERE id = ?;

-- name: CreateDiscountList :one
INSERT INTO discount_lists (name) VALUES (?) RETURNING *;

-- name: UpdateDiscountList :one
UPDATE discount_lists SET name = ? WHERE id = ? RETURNING *;

-- name: SetDiscountListActive :one
UPDATE discount_lists SET is_active = ? WHERE id = ? RETURNING *;

-- name: ListDiscountListItems :many
SELECT * FROM discount_list_items WHERE discount_list_id = ? ORDER BY product_code;

-- name: UpsertDiscountListItem :one
INSERT INTO discount_list_items (discount_list_id, product_code, discount)
VALUES (?, ?, ?)
ON CONFLICT (discount_list_id, product_code) DO UPDATE SET discount = excluded.discount
RETURNING *;

-- name: DeleteDiscountListItem :exec
DELETE FROM discount_list_items WHERE id = ?;

-- name: FindConflictingActiveDiscountListItem :one
SELECT dl.id AS list_id, dl.name AS list_name
FROM discount_list_items dli
JOIN discount_lists dl ON dl.id = dli.discount_list_id
WHERE dl.is_active = 1 AND dl.id != ? AND dli.product_code = ?
LIMIT 1;

-- name: ListActiveDiscountListItems :many
SELECT dli.product_code, dli.discount, dl.id AS list_id, dl.name AS list_name
FROM discount_list_items dli
JOIN discount_lists dl ON dl.id = dli.discount_list_id
WHERE dl.is_active = 1;
