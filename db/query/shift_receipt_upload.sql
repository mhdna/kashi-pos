-- name: CreateShiftReceiptUpload :one
INSERT INTO shift_receipt_uploads (shift_id, token, expires_at, mode, upload_url)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetShiftReceiptUploadByToken :one
SELECT * FROM shift_receipt_uploads
WHERE token = ?
LIMIT 1;

-- name: SwitchShiftReceiptUploadRoute :exec
UPDATE shift_receipt_uploads
SET mode = ?, upload_url = ?
WHERE token = ?;

-- name: MarkShiftReceiptCaptured :exec
UPDATE shift_receipt_uploads
SET file_path = ?, content_type = ?, size_bytes = ?, captured_at = datetime('now')
WHERE id = ?;

-- name: MarkShiftReceiptSynced :exec
UPDATE shift_receipt_uploads
SET synced_at = datetime('now')
WHERE id = ?;

-- name: MarkShiftReceiptUploadAttemptFailed :exec
UPDATE shift_receipt_uploads
SET attempt_count = attempt_count + 1, next_attempt_at = ?
WHERE id = ?;

-- name: SkipShiftReceiptUpload :exec
UPDATE shift_receipt_uploads
SET skipped_reason = ?
WHERE token = ?;

-- name: ListDueShiftReceiptUploads :many
-- Captured on the POS but not yet confirmed by kashi, and off backoff. Only
-- 'local' rows are ever forwarded from here: a 'central' row's photo went
-- straight to kashi, so there's nothing to push. Deliberately no 'failed'
-- filter -- this table never gives up.
SELECT * FROM shift_receipt_uploads
WHERE mode = 'local'
  AND captured_at IS NOT NULL
  AND synced_at IS NULL
  AND skipped_reason = ''
  AND next_attempt_at <= datetime('now')
ORDER BY id
LIMIT ?;

-- name: ShiftCloseSyncedForRef :one
-- Guard for the uploader: kashi only has a branch_shifts row to attach the
-- photo to once the shift's own close has synced (entity_type 'shift_close',
-- client_ref 'shift-<id>').
SELECT EXISTS (
    SELECT 1 FROM sync_outbox
    WHERE client_ref = ? AND status = 'synced'
) AS synced;

-- name: MarkSyncOutboxSyncedByClientRef :exec
-- Used by the hybrid close path: when the POS pushes a shift close directly
-- (to get an upload token back) the queued outbox row is now redundant.
UPDATE sync_outbox
SET status = 'synced', synced_at = datetime('now'), synced_to = ?
WHERE client_ref = ? AND status != 'synced';
