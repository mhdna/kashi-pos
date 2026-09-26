-- Tracks whether the operator has confirmed enabling full-mirror sync
-- (Sync All auto-deactivating local-only products/clients that no longer
-- exist in kashi). NULL = never confirmed, so the next Sync All that finds
-- local-only records must show the one-time confirmation dialog before
-- deactivating anything. Once set, later Sync All runs deactivate silently.
ALTER TABLE settings ADD COLUMN full_mirror_confirmed_at text;
