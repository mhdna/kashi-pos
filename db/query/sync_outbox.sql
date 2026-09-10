-- name: CreateSyncOutboxEntry :one
INSERT INTO sync_outbox (entity_type, client_ref, payload_json)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpsertSyncOutboxEntry :exec
-- Used by the "sync everything" / reconcile flow: a row for this client_ref
-- may already exist (e.g. synced to a previous central server, or marked
-- failed). Re-arm it as pending with a fresh payload rather than colliding
-- on the table-wide UNIQUE(client_ref).
INSERT INTO sync_outbox (entity_type, client_ref, payload_json)
VALUES (?, ?, ?)
ON CONFLICT (client_ref) DO UPDATE SET
    entity_type = excluded.entity_type,
    payload_json = excluded.payload_json,
    status = 'pending',
    attempt_count = 0,
    last_error = '',
    next_attempt_at = datetime('now'),
    synced_to = '';

-- name: ListDueSyncOutboxEntries :many
-- Priority order, not plain FIFO: a "sync everything" run can dump tens of
-- thousands of client/product rows into this table, and a live sale queued
-- afterwards must not wait behind all of them. Invoices/settlements drain
-- first, then the other transactional entities, then the bulk catch-up
-- rows; id breaks ties within a tier so it's still FIFO per priority.
SELECT * FROM sync_outbox
WHERE status = 'pending' AND next_attempt_at <= datetime('now')
ORDER BY
  CASE entity_type
    WHEN 'sales_invoice'      THEN 0
    WHEN 'return_invoice'     THEN 0
    WHEN 'invoice_settlement' THEN 0
    WHEN 'shift_close'        THEN 1
    WHEN 'expense'            THEN 1
    WHEN 'loan'               THEN 1
    WHEN 'visitor_event'      THEN 1
    WHEN 'attendance_events'  THEN 1
    WHEN 'attendance_change'  THEN 1
    ELSE 2
  END,
  id
LIMIT ?;

-- name: MarkSyncOutboxSynced :exec
UPDATE sync_outbox
SET status = 'synced', synced_at = datetime('now'), synced_to = ?
WHERE id = ?;

-- name: MarkSyncOutboxAttemptFailed :exec
UPDATE sync_outbox
SET attempt_count = attempt_count + 1, last_error = ?, next_attempt_at = ?
WHERE id = ?;

-- name: MarkSyncOutboxFailed :exec
-- Terminal state for a row kashi rejected outright (a 4xx, or an entity_type
-- it has no route for). It stays out of the drain batch until an explicit
-- resync re-arms it.
UPDATE sync_outbox
SET status = 'failed', attempt_count = attempt_count + 1, last_error = ?
WHERE id = ?;

-- name: CountPendingSyncOutbox :one
SELECT COUNT(*) FROM sync_outbox WHERE status = 'pending';

-- name: CountSyncOutboxByStatus :one
SELECT COUNT(*) FROM sync_outbox WHERE status = ?;

-- name: CountSyncOutboxConfirmedOnTarget :one
SELECT COUNT(*) FROM sync_outbox WHERE status = 'synced' AND synced_to = ?;

-- name: CountSyncOutboxConfirmedElsewhere :one
SELECT COUNT(*) FROM sync_outbox WHERE status = 'synced' AND synced_to <> ?;

-- name: LatestSyncOutboxSyncedAt :one
SELECT CAST(COALESCE(MAX(synced_at), '') AS text) FROM sync_outbox WHERE status = 'synced';

-- name: SyncStateForInvoices :many
-- Per-invoice sync dot source: joins each invoice to its stable client_ref
-- and whatever outbox row carries it. status/synced_to are NULL when the
-- invoice was never queued (sync was off when it was rung up).
SELECT
    icr.invoice_id   AS invoice_id,
    so.status        AS status,
    so.synced_to     AS synced_to
FROM invoice_client_refs icr
LEFT JOIN sync_outbox so ON so.client_ref = icr.client_ref
WHERE icr.invoice_id IN (sqlc.slice('invoice_ids'));

-- name: ClearSyncCursors :exec
DELETE FROM sync_cursors;

-- name: RequeueSyncOutboxConfirmedElsewhere :execrows
-- Re-arms rows that a previous central server accepted (or that hard-failed)
-- so a "sync everything" against a newly-pointed server re-pushes them.
-- Live 'pending' rows and rows already confirmed on the current target are
-- left alone.
UPDATE sync_outbox
SET status = 'pending', attempt_count = 0, last_error = '',
    next_attempt_at = datetime('now'), synced_to = ''
WHERE (status = 'synced' AND synced_to <> sqlc.arg(current_target))
   OR status = 'failed';

-- name: ListInvoiceClientRefsMap :many
SELECT invoice_id, client_ref FROM invoice_client_refs;
