-- When a pending thank-you is next tried. Retries back off (1, 2, 4... up to
-- 30 minutes) so a bridge or kashi outage of an hour or two doesn't use up
-- the attempts in its first half hour. NULL: not tried yet.
ALTER TABLE whatsapp_pending ADD COLUMN next_attempt_at TIMESTAMP;
