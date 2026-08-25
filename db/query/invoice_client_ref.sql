-- name: GetInvoiceClientRef :one
SELECT client_ref FROM invoice_client_refs WHERE invoice_id = ?;

-- name: CreateInvoiceClientRef :exec
INSERT INTO invoice_client_refs (invoice_id, client_ref) VALUES (?, ?);
