-- When this till told kashi which salesperson made each of its past sales
-- (kashi's id, salespersons.remote_id). NULL until it has; later sales carry
-- the id on their own. See reportInvoiceSalespersons in sync_salesperson_ids.go.
ALTER TABLE settings ADD COLUMN salesperson_ids_reported_at text;
