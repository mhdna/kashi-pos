-- expenses, loans and visitor_events were created (000074/000075/000084)
-- with datetime('now', 'localtime') defaults, after 000066 had settled
-- every other timestamp on UTC. The screens and the kashi re-send path read
-- them as UTC, so they showed and re-sent 3 hours off (in Beirut). Convert
-- the till's own rows to UTC; the code now writes UTC.
--
-- Expenses and loans imported from kashi (PullBranchHistory) were already
-- written in UTC and must not be shifted again. Those are never in the
-- sync outbox, while every expense/loan this till made is, as soon as sync
-- is set up (written with it, or re-sent by "Sync everything"). A till with
-- no kashi has no imported rows, so all of its rows are its own.
UPDATE expenses
SET occurred_at = datetime(occurred_at, 'utc'), created_at = datetime(created_at, 'utc')
WHERE client_ref IN (SELECT client_ref FROM sync_outbox WHERE entity_type = 'expense')
   OR COALESCE((SELECT central_api_url FROM settings LIMIT 1), '') = '';

UPDATE loans
SET occurred_at = datetime(occurred_at, 'utc'), created_at = datetime(created_at, 'utc')
WHERE client_ref IN (SELECT client_ref FROM sync_outbox WHERE entity_type = 'loan')
   OR COALESCE((SELECT central_api_url FROM settings LIMIT 1), '') = '';

-- Visitor events are only ever made on this till.
UPDATE visitor_events
SET occurred_at = datetime(occurred_at, 'utc'), created_at = datetime(created_at, 'utc');
