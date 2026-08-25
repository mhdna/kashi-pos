-- name: GetAttributeByName :one
SELECT * FROM attributes WHERE name = ?;

-- name: ListProductAttributeValues :many
-- For a single-product lookup, where scanning the whole catalog's
-- attributes (as ListAllProductAttributeValues does) would be wasteful.
SELECT a.name AS attribute_name, av.value
FROM products_attributes pa
JOIN attributes a ON a.id = pa.attribute_id
JOIN attributes_values av ON av.id = pa.attribute_value_id
WHERE pa.product_id = ?;

-- name: ListAllProductAttributeValues :many
-- Loads every product's attribute values in one query, same pattern as
-- loadActiveOverrides (pricing_overrides.go): one query per request,
-- regardless of how many products are in the result set, avoiding an N+1
-- per-product lookup in productMap.
SELECT pa.product_id, a.name AS attribute_name, av.value
FROM products_attributes pa
JOIN attributes a ON a.id = pa.attribute_id
JOIN attributes_values av ON av.id = pa.attribute_value_id;

-- name: UpsertAttributeValue :one
INSERT INTO attributes_values (attribute_id, value)
VALUES (?, ?)
ON CONFLICT (attribute_id, value) DO UPDATE SET value = excluded.value
RETURNING *;

-- name: SetProductAttribute :exec
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
VALUES (?, ?, ?)
ON CONFLICT (attribute_id, product_id) DO UPDATE SET attribute_value_id = excluded.attribute_value_id;
