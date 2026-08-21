-- Persists the PIN login lockout so a thief can't dodge the wait by
-- restarting the app. Single row keyed by id=1; locked_until is an RFC3339
-- timestamp (empty string when not locked). lockout_level tracks how many
-- times the 3-attempt threshold has been hit, used to make each subsequent
-- lockout wait longer.
CREATE TABLE IF NOT EXISTS pin_lockout (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	failed_attempts INTEGER NOT NULL DEFAULT 0,
	lockout_level INTEGER NOT NULL DEFAULT 0,
	locked_until TEXT NOT NULL DEFAULT ''
);
