-- name: GetInvoiceClientRef :one
SELECT client_ref FROM invoice_client_refs WHERE invoice_id = ?;

-- name: GetInvoiceIDByClientRef :one
SELECT invoice_id FROM invoice_client_refs WHERE client_ref = ?;

-- name: CreateInvoiceClientRef :exec
INSERT INTO invoice_client_refs (invoice_id, client_ref) VALUES (?, ?);

-- name: DeleteInvoiceClientRef :exec
DELETE FROM invoice_client_refs WHERE invoice_id = ?;
