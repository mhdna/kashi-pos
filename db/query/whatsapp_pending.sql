-- name: SaveWhatsAppPending :exec
INSERT INTO whatsapp_pending (invoice_code, payload) VALUES (?, ?)
ON CONFLICT (invoice_code) DO UPDATE SET payload = excluded.payload;

-- name: DeleteWhatsAppPending :exec
DELETE FROM whatsapp_pending WHERE invoice_code = ?;

-- name: ListWhatsAppPending :many
-- Oldest first; the caller drops the ones too old to still send.
SELECT * FROM whatsapp_pending ORDER BY created_at;

-- name: BumpWhatsAppPending :exec
UPDATE whatsapp_pending SET attempts = attempts + 1 WHERE invoice_code = ?;
