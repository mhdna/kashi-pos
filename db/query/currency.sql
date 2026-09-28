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

-- name: DeactivateCurrenciesNotIn :exec
-- The reconcile half of the currency sync: a currency kashi no longer has
-- stops being offered. is_default = false guards the local USD anchor, which
-- is how every amount in this database is stored -- it is structural, not a
-- currency kashi gets an opinion about, and the same guard is already on
-- UpdateCurrency and SetCurrencyActive above.
UPDATE currencies
SET is_active = false
WHERE code NOT IN (sqlc.slice('codes'))
  AND is_default = false;

-- name: CountCashboxAccountsByCurrency :one
SELECT COUNT(*) FROM cashbox_accounts WHERE currency_code = ?;
