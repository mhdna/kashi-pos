-- Google Maps, WhatsApp and An ad or flyer go (kashi migration 000262): the
-- starting options are Social media, A friend or family and Walking by.
DELETE FROM client_sources
WHERE uid IN ('6f1c2a00-5e1d-4c7a-9b00-000000000006', '6f1c2a00-5e1d-4c7a-9b00-000000000007', '6f1c2a00-5e1d-4c7a-9b00-000000000008');
