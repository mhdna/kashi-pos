-- Hybrid upload flow: a shift_receipt_uploads row now records which route
-- its QR points at.
--   'local'   -- phone uploads to this POS's own server (shift_upload_server.go);
--                the row is then forwarded to kashi by sync_shift_receipts.go.
--   'central' -- phone uploads straight to kashi (/shift_uploads/<token>); the
--                POS only polls kashi for completion and never forwards.
-- upload_url is the exact link baked into the QR, stored so the close-shift
-- dialog and any "switch to local" fallback stay consistent.
ALTER TABLE shift_receipt_uploads ADD COLUMN mode text NOT NULL DEFAULT 'local';
ALTER TABLE shift_receipt_uploads ADD COLUMN upload_url text NOT NULL DEFAULT '';
