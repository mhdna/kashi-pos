-- name: UpsertInvoiceDiscount :exec
INSERT INTO invoice_discounts (uid, name, min_total, kind, percent, amount, valid_from, valid_to)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (uid) DO UPDATE SET
    name = excluded.name, min_total = excluded.min_total, kind = excluded.kind,
    percent = excluded.percent, amount = excluded.amount,
    valid_from = excluded.valid_from, valid_to = excluded.valid_to;

-- name: DeleteInvoiceDiscount :exec
DELETE FROM invoice_discounts WHERE uid = ?;

-- name: ListInvoiceDiscounts :many
SELECT * FROM invoice_discounts ORDER BY min_total, uid;

-- name: SetInvoiceDiscount :exec
UPDATE invoices SET invoice_discount = ?, invoice_discount_name = ? WHERE id = ?;
