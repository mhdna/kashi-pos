-- The kashi central backend base URL this branch syncs against, e.g.
-- https://kashi.example.com. Empty string means sync is disabled for this
-- install — it should behave exactly as it does today, with no outbound
-- calls at all.
ALTER TABLE settings ADD COLUMN central_api_url TEXT NOT NULL DEFAULT '';
