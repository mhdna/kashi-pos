-- name: ListCurrencies :many
SELECT * FROM currencies ORDER BY code;

-- name: GetDefaultCurrency :one
SELECT * FROM currencies WHERE is_default = true;

-- name: GetCurrencyByCode :one
SELECT * FROM currencies WHERE code = ?;

-- name: CreateCurrency :one
INSERT INTO currencies (code, name, symbol, units_per_usd_micros, cash_rounding_unit)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateCurrency :one
UPDATE currencies
SET name = ?, symbol = ?, units_per_usd_micros = ?, cash_rounding_unit = ?
WHERE code = ? AND is_default = false
RETURNING *;

-- name: SetCurrencyActive :one
UPDATE currencies SET is_active = ? WHERE code = ? AND is_default = false
RETURNING *;

-- name: CountCashboxAccountsByCurrency :one
SELECT COUNT(*) FROM cashbox_accounts WHERE currency_code = ?;
