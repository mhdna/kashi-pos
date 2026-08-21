-- Several created_at/closed_at columns were populated with
-- datetime('now', 'localtime') (local wall-clock time) while every other
-- timestamp column in the database uses datetime('now') (UTC). This mix
-- made it impossible for application code to tell, from the column alone,
-- which convention a given value used - which produced wrong displayed and
-- computed dates (e.g. the on-hold dialog, shift monthly totals). This
-- migration converts existing local-time values in those columns to UTC so
-- every created_at/closed_at column in the database is consistently UTC.
-- Application code and queries are updated alongside this migration to
-- write UTC going forward.
UPDATE shifts SET created_at = datetime(created_at, 'utc')
    WHERE created_at IS NOT NULL AND created_at != '';
UPDATE shifts SET closed_at = datetime(closed_at, 'utc')
    WHERE closed_at IS NOT NULL AND closed_at != '';
UPDATE client_account_entries SET created_at = datetime(created_at, 'utc')
    WHERE created_at IS NOT NULL AND created_at != '';
UPDATE client_account_payments SET created_at = datetime(created_at, 'utc')
    WHERE created_at IS NOT NULL AND created_at != '';
UPDATE shift_balance_replacements SET created_at = datetime(created_at, 'utc')
    WHERE created_at IS NOT NULL AND created_at != '';

-- attendance_change_requests.created_at/reviewed_at were written as
-- RFC3339 UTC strings (e.g. "2026-08-21T14:00:00Z") while every other
-- timestamp column uses the plain SQL datetime format ("2026-08-21
-- 14:00:00"). datetime(x) normalizes any RFC3339 input to that plain UTC
-- format, and is a no-op on values already in it.
UPDATE attendance_change_requests SET created_at = datetime(created_at)
    WHERE created_at IS NOT NULL AND created_at != '';
UPDATE attendance_change_requests SET reviewed_at = datetime(reviewed_at)
    WHERE reviewed_at IS NOT NULL AND reviewed_at != '';
