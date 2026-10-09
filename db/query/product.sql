-- name: GetProductByID :one
SELECT * FROM products WHERE id = ?;

-- name: ListProductsByIDs :many
SELECT * FROM products WHERE id IN (sqlc.slice('ids'));

-- name: GetProductByBarcode :one
SELECT * FROM products WHERE barcode = ?;

-- name: GetProductByCode :one
SELECT * FROM products WHERE code = ? ORDER BY id LIMIT 1;

-- name: ListActiveProductsByCode :many
-- Codes repeat across colour/size variants, so a code can name several
-- products; the scan field offers them all instead of picking one. Case
-- doesn't matter: a cashier typing "1579b" means 1579B.
SELECT * FROM products WHERE code = ? COLLATE NOCASE AND is_active = 1 ORDER BY id;

-- name: ListProducts :many
SELECT * FROM products ORDER BY id;

-- name: ListActiveProducts :many
SELECT * FROM products WHERE is_active = 1 ORDER BY id;

-- name: SearchProductsByName :many
SELECT * FROM products WHERE name LIKE ? ORDER BY id;

-- name: ListProductsPage :many
-- search_pattern must already be wrapped in '%...%' by the caller.
--
-- Inactive products are excluded, matching ListProductsAfter below: a product
-- is deactivated because kashi retired it (DeactivateProductsByBarcode, driven
-- by the full-mirror check), so offering it in a search only lets a cashier
-- put something unsellable in the cart.
SELECT * FROM products
WHERE is_active = 1
  AND (name || ' ' || code || ' ' || barcode)
      LIKE sqlc.arg(search_pattern)
ORDER BY id
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListProductsPageUnfiltered :many
SELECT * FROM products
WHERE is_active = 1
ORDER BY id
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: ListProductsAfter :many
-- search_pattern must already be wrapped in '%...%' by the caller.
SELECT * FROM products
WHERE id > sqlc.arg(after_id)
  AND is_active = 1
  AND (name || ' ' || code || ' ' || barcode)
      LIKE sqlc.arg(search_pattern)
ORDER BY id
LIMIT sqlc.arg(limit);

-- name: ListProductsAfterUnfiltered :many
SELECT * FROM products
WHERE id > sqlc.arg(after_id)
  AND is_active = 1
ORDER BY id
LIMIT sqlc.arg(limit);

-- name: CountProducts :one
-- search_pattern must already be wrapped in '%...%' by the caller. Counts the
-- same rows ListProductsPage returns, so the total under a paged search never
-- promises more results than the pages can show.
SELECT COUNT(*) FROM products
WHERE is_active = 1
  AND (name || ' ' || code || ' ' || barcode)
      LIKE sqlc.arg(search_pattern);

-- name: CountAllProducts :one
SELECT COUNT(*) FROM products WHERE is_active = 1;

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

-- name: DeactivateProductsByBarcode :exec
UPDATE products SET is_active = 0 WHERE barcode IN (sqlc.slice('barcodes'));

-- name: ListQuickAccessProducts :many
-- A pinned product that has since been deactivated centrally drops off the
-- quick-access grid rather than sitting there unsellable.
SELECT p.*
FROM quick_access_items q
JOIN products p ON p.id = q.product_id
WHERE p.is_active = 1
ORDER BY q.position, q.created_at, q.product_id;

-- name: AddQuickAccessProduct :exec
INSERT INTO quick_access_items (product_id, position)
VALUES (?, COALESCE((SELECT MAX(position) + 1 FROM quick_access_items), 0))
ON CONFLICT (product_id) DO NOTHING;

-- name: RemoveQuickAccessProduct :exec
DELETE FROM quick_access_items WHERE product_id = ?;

-- name: SetQuickAccessPosition :exec
UPDATE quick_access_items SET position = ? WHERE product_id = ?;

-- name: ListQuickAccessColors :many
SELECT product_id, color, text_color FROM quick_access_items WHERE color <> '' OR text_color <> '';

-- name: SetQuickAccessColor :exec
UPDATE quick_access_items SET color = ?, text_color = ? WHERE product_id = ?;

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

-- name: GetProductByUID :one
SELECT * FROM products WHERE uid = ?;

-- name: SetProductUID :exec
UPDATE products SET uid = ? WHERE id = ?;

-- name: SetProductBarcodeInactive :exec
-- Frees a barcode held by a stale local-only product so kashi's product
-- with that barcode can take it; the stale row keeps its history.
UPDATE products SET barcode = ?, is_active = 0 WHERE id = ?;

-- name: SetProductRemoteUpdatedAt :exec
UPDATE products SET remote_updated_at = ? WHERE id = ?;

-- name: ListProductSyncVersions :many
SELECT id, uid, remote_updated_at, is_active FROM products WHERE uid IS NOT NULL;

-- name: DeactivateProductsByIDs :exec
UPDATE products SET is_active = 0 WHERE id IN (sqlc.slice('ids'));

-- name: ProductIsReferenced :one
-- History that must keep resolving to this product: sold/returned lines,
-- held invoices, transfers, or stock on hand. Such a product is deactivated,
-- never deleted.
SELECT CAST(EXISTS (
    SELECT 1 FROM invoice_products ip WHERE ip.product_id = sqlc.arg(id)
    UNION ALL SELECT 1 FROM hold_invoice_items hi WHERE hi.product_id = sqlc.arg(id)
    UNION ALL SELECT 1 FROM transfer_items ti WHERE ti.product_id = sqlc.arg(id)
    UNION ALL SELECT 1 FROM inventory_items ii WHERE ii.product_id = sqlc.arg(id) AND ii.quantity != 0
) AS INTEGER) AS referenced;

-- name: DeleteProductOwnedRows :exec
-- Rows that only describe the product (not history), removed with it.
DELETE FROM products_attributes WHERE product_id = sqlc.arg(id);

-- name: DeleteProductInventoryRows :exec
DELETE FROM inventory_items WHERE product_id = sqlc.arg(id) AND quantity = 0;

-- name: DeleteProductQuickAccess :exec
DELETE FROM quick_access_items WHERE product_id = sqlc.arg(id);

-- name: DeleteProductOverrides :exec
DELETE FROM price_overrides WHERE product_id = sqlc.arg(id);

-- name: DeleteProductDiscountOverrides :exec
DELETE FROM discount_overrides WHERE product_id = sqlc.arg(id);

-- name: SetProductActive :exec
UPDATE products SET is_active = ? WHERE id = ?;

-- name: ListProductsForManagement :many
-- Standalone product screen: every product, active or not, paged by id.
SELECT * FROM products
WHERE id > sqlc.arg(after_id)
  AND (sqlc.arg(search) = '' OR (name || ' ' || code || ' ' || barcode) LIKE '%' || sqlc.arg(search) || '%')
ORDER BY id
LIMIT sqlc.arg(limit);
