-- Central's starting options, reworked (kashi migration 000261): SMS and
-- Other go, the friend option takes family too, and Google Maps, WhatsApp and
-- an ad or flyer join. Central's list replaces this at the next sync anyway;
-- this is what a till shows before it. A customer's saved answer is kept.
DELETE FROM client_sources
WHERE uid IN ('6f1c2a00-5e1d-4c7a-9b00-000000000002', '6f1c2a00-5e1d-4c7a-9b00-000000000005');

UPDATE client_sources SET name = 'A friend or family'
WHERE uid = '6f1c2a00-5e1d-4c7a-9b00-000000000003' AND name = 'A friend';

INSERT OR IGNORE INTO client_sources (uid, name, sort_order) VALUES
  ('6f1c2a00-5e1d-4c7a-9b00-000000000006', 'Google Maps', 6),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000007', 'WhatsApp', 7),
  ('6f1c2a00-5e1d-4c7a-9b00-000000000008', 'An ad or flyer', 8);
