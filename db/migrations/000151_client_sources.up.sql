-- "Where did the customer hear about us?", asked when a cashier adds a new
-- customer. The options come from central (entity "client_sources"); these
-- are central's starting options under the same uids, so the till can ask
-- before its first sync and its answers mean the same thing there.
CREATE TABLE IF NOT EXISTS client_sources (
  uid text PRIMARY KEY,
  name text NOT NULL,
  sort_order integer NOT NULL DEFAULT 0,
  is_active integer NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO client_sources (uid, name, sort_order) VALUES
  ('6f1c2a00-5e1d-4c7a-9b00-000000000001', 'Social media', 1),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000002', 'SMS', 2),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000003', 'A friend', 3),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000004', 'Walking by', 4),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000005', 'Other', 5);

-- The answer, sent to central with the client ('' = not asked).
ALTER TABLE clients ADD COLUMN source_uid text NOT NULL DEFAULT '';
