-- name: GetInventoryByID :one
SELECT * FROM inventories WHERE id = ?;

-- name: ListInventories :many
SELECT * FROM inventories ORDER BY id;

-- name: CreateInventory :one
INSERT INTO inventories (name) VALUES (?) RETURNING *;

-- name: UpdateInventory :one
UPDATE inventories SET name = ? WHERE id = ? RETURNING *;

-- name: DeleteInventory :exec
DELETE FROM inventories WHERE id = ?;

-- name: GetInventoryItem :one
SELECT * FROM inventory_items WHERE inventory_id = ? AND product_id = ?;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items WHERE inventory_id = ?;

-- name: UpsertInventoryItem :exec
INSERT INTO inventory_items (inventory_id, product_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (inventory_id, product_id) DO UPDATE SET quantity = excluded.quantity;

-- name: AddInventoryProductQuantity :exec
INSERT INTO inventory_items (inventory_id, product_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (inventory_id, product_id) DO UPDATE SET quantity = inventory_items.quantity + excluded.quantity;

-- name: DecreaseInventoryProductQuantity :execresult
UPDATE inventory_items
SET quantity = quantity - ?
WHERE inventory_id = ? AND product_id = ? AND quantity >= ?;

-- name: DeleteInventoryItem :exec
DELETE FROM inventory_items WHERE inventory_id = ? AND product_id = ?;

-- name: GetLastInvoiceInventoryID :one
SELECT inventory_id FROM invoices ORDER BY id DESC LIMIT 1;
