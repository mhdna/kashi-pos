-- Sale options mirrored from kashi -- see 000126_sale_options.up.sql.

-- name: DeleteProductSaleOptions :exec
DELETE FROM product_sale_options WHERE product_id = ?;

-- name: InsertProductSaleOption :exec
INSERT INTO product_sale_options (product_id, name, sort_order) VALUES (?, ?, ?)
ON CONFLICT (product_id, name) DO UPDATE SET sort_order = excluded.sort_order;

-- name: ListAllProductSaleOptions :many
-- Keyed by uid so loadActivePricing can hold them beside the discounts.
SELECT p.uid AS product_uid, pso.name
FROM product_sale_options pso
JOIN products p ON p.id = pso.product_id
WHERE p.uid IS NOT NULL
ORDER BY pso.product_id, pso.sort_order, pso.name;

-- name: DeleteDiscountListOptionItemsForList :exec
DELETE FROM discount_list_option_items WHERE discount_list_id = ?;

-- name: InsertDiscountListOptionItem :exec
INSERT INTO discount_list_option_items (discount_list_id, product_uid, sale_option, discount)
VALUES (?, ?, ?, ?)
ON CONFLICT (discount_list_id, product_uid, sale_option) DO UPDATE SET discount = excluded.discount;

-- name: ListActiveDiscountListOptionItems :many
-- See ListActiveDiscountListItems for why this is ordered by is_default DESC.
SELECT dloi.product_uid, dloi.sale_option, dloi.discount, dl.id AS list_id, dl.name AS list_name, dl.is_special
FROM discount_list_option_items dloi
JOIN discount_lists dl ON dl.id = dloi.discount_list_id
WHERE dl.is_active = 1
ORDER BY dloi.product_uid, dl.is_default DESC;
