-- Shared identity with kashi (see kashi's 000093_sync_uid). Every client and
-- product carries a uid that is the same on kashi and on every till; sync
-- payloads reference rows by uid, never by this database's integer id.
--
-- New rows get a random v4 uid from the triggers below, so no insert path
-- has to remember to mint one. Rows that come from kashi then overwrite it
-- with kashi's uid (see applySyncedClient / applySyncedProductVariant).
ALTER TABLE clients ADD COLUMN uid text;
ALTER TABLE products ADD COLUMN uid text;

UPDATE clients SET uid =
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
    lower(hex(randomblob(6)))
WHERE uid IS NULL;

UPDATE products SET uid =
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
    lower(hex(randomblob(6)))
WHERE uid IS NULL;

CREATE UNIQUE INDEX idx_clients_uid ON clients(uid);
CREATE UNIQUE INDEX idx_products_uid ON products(uid);

CREATE TRIGGER clients_mint_uid AFTER INSERT ON clients
WHEN NEW.uid IS NULL
BEGIN
    UPDATE clients SET uid =
        lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
        substr(lower(hex(randomblob(2))), 2) || '-' ||
        substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
        lower(hex(randomblob(6)))
    WHERE id = NEW.id;
END;

CREATE TRIGGER products_mint_uid AFTER INSERT ON products
WHEN NEW.uid IS NULL
BEGIN
    UPDATE products SET uid =
        lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
        substr(lower(hex(randomblob(2))), 2) || '-' ||
        substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
        lower(hex(randomblob(6)))
    WHERE id = NEW.id;
END;
