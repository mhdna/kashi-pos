-- name: GetProductByID :one
SELECT * FROM products WHERE id = ?;

-- name: GetProductByBarcode :one
SELECT * FROM products WHERE barcode = ?;

-- name: GetProductByCode :one
SELECT * FROM products WHERE code = ?;

-- name: ListProducts :many
SELECT * FROM products ORDER BY id;

-- name: SearchProductsByName :many
SELECT * FROM products WHERE name LIKE ? ORDER BY id;

-- name: CreateProduct :one
INSERT INTO products (barcode, code, name, description, is_active, price, discount)
VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET barcode = ?, code = ?, name = ?, description = ?,
    is_active = ?, price = ?, discount = ?
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
