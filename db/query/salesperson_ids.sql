-- name: ListInvoiceSalespersonRemoteIDs :many
-- Every invoice made by a salesperson kashi knows, with kashi's id for them.
SELECT i.invoice_code, s.remote_id
FROM invoices i
JOIN salespersons s ON s.id = i.salesperson_id
WHERE s.remote_id IS NOT NULL AND s.remote_id > 0 AND i.invoice_code <> ''
ORDER BY i.id;

-- name: SetSalespersonIDsReportedAt :exec
UPDATE settings SET salesperson_ids_reported_at = ?;
