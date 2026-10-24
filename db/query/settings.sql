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

-- name: SetFullMirrorConfirmedAt :exec
UPDATE settings SET full_mirror_confirmed_at = ?;

-- name: GetFeatureFlag :one
SELECT enabled FROM feature_flags WHERE key = ?;

-- name: SetFeatureFlag :exec
INSERT INTO feature_flags (key, enabled) VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET enabled = excluded.enabled;

-- name: UpdateSplitDisplay :exec
-- Which currency leads the till's split total, in which order, and at what
-- denomination the primary is split (0 = fall back to that currency's own
-- cash_rounding_unit -- see 000100). Narrow like UpdateTheme rather than
-- folded into UpdateSettings: the admin command and the settings page both
-- set these on their own.
UPDATE settings SET split_primary_currency = ?, split_primary_first = ?, split_primary_step = ?;

-- name: SetHomeBranchCode :exec
UPDATE settings SET home_branch_code = ?;

-- name: SetOversellMode :exec
UPDATE settings SET oversell_mode = ?;

-- name: SetPriceEditCodes :exec
UPDATE settings SET price_edit_codes = ?;

-- name: SetUnknownProductBarcode :exec
UPDATE settings SET unknown_product_barcode = ?;

-- name: SetProductSearchHiddenFields :exec
UPDATE settings SET product_search_hidden_fields = ?;

-- name: SetReceiptLook :exec
-- Narrow like UpdateTheme: the settings page and kashi's update_settings
-- command both set the printed invoice's theme and layout on their own.
UPDATE settings SET receipt_theme = ?, receipt_layout = ?;
