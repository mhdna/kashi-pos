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
