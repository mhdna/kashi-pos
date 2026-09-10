-- name: GetSettings :one
SELECT * FROM settings LIMIT 1;

-- name: UpdateSettings :exec
UPDATE settings
SET branch_name = ?, branch_code = ?, api_key = ?, printer_id = ?, printer_size = ?,
    receipt_printer = ?, screen_port = ?, receipt_width = ?, receipt_height = ?,
    theme = ?, tax_rate = ?, rounding_mode = ?, rounding_currency = ?, receipt_enabled = ?, sales_inventory_id = ?, exchange_rate = ?, max_overpayment_cents = ?, exchange_window_hours = ?,
    market_name = ?, market_phone = ?, market_description = ?, return_policy = ?, website = ?, instagram = ?, social_platforms = ?, social_handles = ?,
    receipt_font = ?, receipt_title_size = ?, receipt_body_size = ?, receipt_cutoff = ?, receipt_body_font = ?, branch_phone = ?, counter_number = ?;

-- name: UpdateDefaultOpeningFloat :exec
UPDATE settings SET default_opening_float_usd = ?, default_opening_float_lbp = ?;

-- name: UpdateTheme :exec
UPDATE settings SET theme = ?;

-- name: UpsertSettings :exec
INSERT INTO settings (branch_name, branch_code, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id, exchange_rate, max_overpayment_cents, exchange_window_hours, market_name, market_phone, market_description, return_policy, website, instagram, social_platforms, social_handles, receipt_font, receipt_title_size, receipt_body_size, receipt_cutoff, receipt_body_font, branch_phone, counter_number)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCentralAPIURL :exec
UPDATE settings SET central_api_url = ?;

-- name: SetShiftReceiptUploadMode :exec
UPDATE settings SET shift_receipt_upload_mode = ?;

-- name: GetFeatureFlag :one
SELECT enabled FROM feature_flags WHERE key = ?;

-- name: SetWhatsAppSettings :exec
UPDATE settings SET
    whatsapp_enabled = ?,
    whatsapp_api_url = ?,
    whatsapp_api_key = ?,
    whatsapp_session = ?,
    whatsapp_message = ?,
    whatsapp_review_url = ?;

-- name: SetFeatureFlag :exec
INSERT INTO feature_flags (key, enabled) VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET enabled = excluded.enabled;
