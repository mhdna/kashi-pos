-- name: GetSettings :one
SELECT * FROM settings LIMIT 1;

-- name: UpdateSettings :exec
UPDATE settings
SET branch_name = ?, branch_code = ?, api_key = ?, printer_id = ?, printer_size = ?,
    receipt_printer = ?, screen_port = ?, receipt_width = ?, receipt_height = ?,
    theme = ?, tax_rate = ?, rounding_mode = ?, receipt_enabled = ?, sales_inventory_id = ?, exchange_rate = ?;

-- name: UpdateTheme :exec
UPDATE settings SET theme = ?;

-- name: UpsertSettings :exec
INSERT INTO settings (branch_name, branch_code, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id, exchange_rate)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
