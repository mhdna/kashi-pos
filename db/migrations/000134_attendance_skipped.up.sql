-- Punches the attendance device logged that the till doesn't import (the
-- device user isn't linked to any salesperson), already reported to kashi.
-- The device sends its whole history on every pull, so this keeps each one
-- from being reported again.
CREATE TABLE IF NOT EXISTS attendance_skipped_sent (
    ref TEXT PRIMARY KEY,
    sent_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
