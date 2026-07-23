-- name: ListCurrencies :many
SELECT * FROM currencies ORDER BY code;

-- name: GetDefaultCurrency :one
SELECT * FROM currencies WHERE is_default = true;
