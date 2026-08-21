-- name: ListPriceLists :many
SELECT * FROM price_lists ORDER BY name;

-- name: GetPriceListByID :one
SELECT * FROM price_lists WHERE id = ?;

-- name: CreatePriceList :one
INSERT INTO price_lists (name) VALUES (?) RETURNING *;

-- name: UpdatePriceList :one
UPDATE price_lists SET name = ? WHERE id = ? RETURNING *;

-- name: SetPriceListActive :one
UPDATE price_lists SET is_active = ? WHERE id = ? RETURNING *;

-- name: ListPriceListItems :many
SELECT * FROM price_list_items WHERE price_list_id = ? ORDER BY product_code;

-- name: UpsertPriceListItem :one
INSERT INTO price_list_items (price_list_id, product_code, price)
VALUES (?, ?, ?)
ON CONFLICT (price_list_id, product_code) DO UPDATE SET price = excluded.price
RETURNING *;

-- name: DeletePriceListItem :exec
DELETE FROM price_list_items WHERE id = ?;

-- name: FindConflictingActivePriceListItem :one
SELECT pl.id AS list_id, pl.name AS list_name
FROM price_list_items pli
JOIN price_lists pl ON pl.id = pli.price_list_id
WHERE pl.is_active = 1 AND pl.id != ? AND pli.product_code = ?
LIMIT 1;

-- name: ListActivePriceListItems :many
SELECT pli.product_code, pli.price, pl.id AS list_id, pl.name AS list_name
FROM price_list_items pli
JOIN price_lists pl ON pl.id = pli.price_list_id
WHERE pl.is_active = 1;
