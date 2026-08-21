-- name: GetProductByID :one
SELECT * FROM products WHERE id = ?;

-- name: GetProductByBarcode :one
SELECT * FROM products WHERE barcode = ?;

-- name: GetProductByCode :one
SELECT * FROM products WHERE code = ? ORDER BY id LIMIT 1;

-- name: ListProducts :many
SELECT * FROM products ORDER BY id;

-- name: SearchProductsByName :many
SELECT * FROM products WHERE name LIKE ? ORDER BY id;

-- name: ListProductsPage :many
SELECT * FROM products
WHERE name LIKE '%' || sqlc.arg(search) || '%'
   OR code LIKE '%' || sqlc.arg(search) || '%'
   OR barcode LIKE '%' || sqlc.arg(search) || '%'
   OR color LIKE '%' || sqlc.arg(search) || '%'
   OR size LIKE '%' || sqlc.arg(search) || '%'
ORDER BY id
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListProductsPageUnfiltered :many
SELECT * FROM products
ORDER BY id
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListProductsAfter :many
SELECT * FROM products
WHERE id > sqlc.arg(after_id)
  AND (
       name LIKE '%' || sqlc.arg(search) || '%'
    OR code LIKE '%' || sqlc.arg(search) || '%'
    OR barcode LIKE '%' || sqlc.arg(search) || '%'
    OR color LIKE '%' || sqlc.arg(search) || '%'
    OR size LIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY id
LIMIT sqlc.arg(limit);

-- name: ListProductsAfterUnfiltered :many
SELECT * FROM products
WHERE id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(limit);

-- name: CountProducts :one
SELECT COUNT(*) FROM products
WHERE name LIKE '%' || sqlc.arg(search) || '%'
   OR code LIKE '%' || sqlc.arg(search) || '%'
   OR barcode LIKE '%' || sqlc.arg(search) || '%'
   OR color LIKE '%' || sqlc.arg(search) || '%'
   OR size LIKE '%' || sqlc.arg(search) || '%';

-- name: CountAllProducts :one
SELECT COUNT(*) FROM products;

-- name: CreateProduct :one
INSERT INTO products (barcode, code, name, description, is_active, price, discount, color, size, currency_code, brand, family, kind, season, product_year)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET barcode = ?, code = ?, name = ?, description = ?,
    is_active = ?, price = ?, discount = ?, color = ?, size = ?, currency_code = ?,
    brand = ?, family = ?, kind = ?, season = ?, product_year = ?
WHERE id = ? RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products WHERE id = ?;

-- name: ListQuickAccessProducts :many
SELECT p.*
FROM quick_access_items q
JOIN products p ON p.id = q.product_id
ORDER BY q.position, q.created_at, q.product_id;

-- name: AddQuickAccessProduct :exec
INSERT INTO quick_access_items (product_id, position)
VALUES (?, COALESCE((SELECT MAX(position) + 1 FROM quick_access_items), 0))
ON CONFLICT (product_id) DO NOTHING;

-- name: RemoveQuickAccessProduct :exec
DELETE FROM quick_access_items WHERE product_id = ?;

-- name: SetQuickAccessPosition :exec
UPDATE quick_access_items SET position = ? WHERE product_id = ?;

-- name: GetPriceOverride :one
SELECT * FROM price_overrides WHERE id = ?;

-- name: ListPriceOverridesByProduct :many
SELECT * FROM price_overrides WHERE product_id = ?;

-- name: CreatePriceOverride :one
INSERT INTO price_overrides (name, product_id, price) VALUES (?, ?, ?) RETURNING *;

-- name: DeletePriceOverride :exec
DELETE FROM price_overrides WHERE id = ?;

-- name: GetDiscountOverride :one
SELECT * FROM discount_overrides WHERE id = ?;

-- name: ListDiscountOverridesByProduct :many
SELECT * FROM discount_overrides WHERE product_id = ?;

-- name: CreateDiscountOverride :one
INSERT INTO discount_overrides (name, product_id, discount) VALUES (?, ?, ?) RETURNING *;

-- name: DeleteDiscountOverride :exec
DELETE FROM discount_overrides WHERE id = ?;
