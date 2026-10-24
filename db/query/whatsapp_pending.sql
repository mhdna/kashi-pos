-- name: SaveWhatsAppPending :exec
INSERT INTO whatsapp_pending (invoice_code, payload) VALUES (?, ?)
ON CONFLICT (invoice_code) DO UPDATE SET payload = excluded.payload;

-- name: DeleteWhatsAppPending :exec
DELETE FROM whatsapp_pending WHERE invoice_code = ?;

-- name: ListWhatsAppPending :many
-- Oldest first; the caller drops the ones too old to still send.
SELECT * FROM whatsapp_pending ORDER BY created_at;

-- name: BumpWhatsAppPending :exec
-- One more failed try; the next waits `delay` (a SQLite modifier such as
-- '+4 minutes').
UPDATE whatsapp_pending
SET attempts = attempts + 1, next_attempt_at = datetime('now', sqlc.arg(delay))
WHERE invoice_code = sqlc.arg(invoice_code);
