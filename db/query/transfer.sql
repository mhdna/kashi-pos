-- name: GetAssetByID :one
SELECT * FROM assets WHERE id = ?;

-- name: ListAssets :many
SELECT * FROM assets ORDER BY id;

-- name: CreateAsset :one
INSERT INTO assets (name, description) VALUES (?, ?) RETURNING *;

-- name: UpdateAsset :one
UPDATE assets SET name = ?, description = ? WHERE id = ? RETURNING *;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = ?;

-- name: GetTransferByID :one
SELECT * FROM transfers WHERE id = ?;

-- name: ListTransfers :many
SELECT * FROM transfers ORDER BY id;

-- name: CreateTransfer :one
INSERT INTO transfers (from_inventory_id, to_inventory_id, type)
VALUES (?, ?, ?) RETURNING *;

-- name: GetTransferItemByID :one
SELECT * FROM transfer_items WHERE id = ?;

-- name: ListTransferItems :many
SELECT * FROM transfer_items WHERE transfer_id = ?;

-- name: CreateTransferItem :exec
INSERT INTO transfer_items (transfer_id, product_id, asset_id, quantity)
VALUES (?, ?, ?, ?);

-- name: DeleteTransferItems :exec
DELETE FROM transfer_items WHERE transfer_id = ?;
