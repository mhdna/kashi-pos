-- Quick Access can come from kashi instead of this till (kashi migration
-- 000110). quick_access_mode is this register's choice:
--   'till'   quick_access_items, edited here (the default -- unchanged)
--   'kashi'  central_quick_access_items, kashi's branch list, read-only here
-- Either side can flip it; quick_access_mode_changed_at (RFC3339) is when
-- the current value was chosen, and the later flip wins (see
-- sync_quick_access.go).
ALTER TABLE settings ADD COLUMN quick_access_mode text NOT NULL DEFAULT 'till'
    CHECK (quick_access_mode IN ('till', 'kashi'));
ALTER TABLE settings ADD COLUMN quick_access_mode_changed_at text NOT NULL DEFAULT '1970-01-01T00:00:00Z';
-- 1 once kashi has accepted this till's own list as a candidate to seed its
-- branch list (it only uses the first one it gets), so it's sent once.
ALTER TABLE settings ADD COLUMN quick_access_reported integer NOT NULL DEFAULT 0;

-- kashi's branch list, as last pulled. Kept beside this till's own list
-- (quick_access_items) rather than replacing it, so switching back to
-- 'till' brings the till's own cards back. Keyed by product uid: kashi
-- doesn't know this till's local product ids.
CREATE TABLE central_quick_access_items (
    product_uid text PRIMARY KEY,
    position integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    text_color text NOT NULL DEFAULT ''
);
