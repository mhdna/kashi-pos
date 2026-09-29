-- Price and discount lists stop being this till's own and become a read-only
-- mirror of kashi's (see kashi's 000094). Three changes.
--
-- 1. Lists get the shared uid identity, same as clients and products in
--    000095, plus is_default -- the default list is the base every price and
--    discount falls back to.
--
-- 2. Items are re-keyed from product_code to product_uid. A code is not
--    unique here (several variants of one product share it), so keying by
--    code could not express two colours of a shirt at different prices; the
--    uid identifies exactly one sellable row, on this till and on kashi.
--
-- 3. Every pre-existing local list is deactivated. It is NOT deleted: the
--    rows are kept so the old contents can still be read, but from now on
--    only lists kashi sends may price anything, and leaving a locally-made
--    list active would let this till quietly override central pricing --
--    exactly what this change exists to stop. The real lists arrive on the
--    next sync and carry kashi's uid.

ALTER TABLE price_lists ADD COLUMN uid text;
ALTER TABLE price_lists ADD COLUMN is_default integer NOT NULL DEFAULT 0;
ALTER TABLE discount_lists ADD COLUMN uid text;
ALTER TABLE discount_lists ADD COLUMN is_default integer NOT NULL DEFAULT 0;

UPDATE price_lists SET uid =
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
    lower(hex(randomblob(6)))
WHERE uid IS NULL;

UPDATE discount_lists SET uid =
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
    lower(hex(randomblob(6)))
WHERE uid IS NULL;

CREATE UNIQUE INDEX idx_price_lists_uid ON price_lists(uid);
CREATE UNIQUE INDEX idx_discount_lists_uid ON discount_lists(uid);


-- SQLite can't drop or re-type a column in place, so the item tables are
-- rebuilt. A row whose product_code matches no product is dropped: there is
-- nothing sellable for it to price.
CREATE TABLE price_list_items_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    price_list_id bigint NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_uid text NOT NULL,
    price bigint NOT NULL,
    created_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE(price_list_id, product_uid)
);

INSERT OR IGNORE INTO price_list_items_new (price_list_id, product_uid, price, created_at)
SELECT pli.price_list_id, p.uid, pli.price, pli.created_at
FROM price_list_items pli
JOIN products p ON p.code = pli.product_code;

DROP TABLE price_list_items;
ALTER TABLE price_list_items_new RENAME TO price_list_items;
CREATE INDEX idx_price_list_items_product_uid ON price_list_items(product_uid);


CREATE TABLE discount_list_items_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    discount_list_id bigint NOT NULL REFERENCES discount_lists(id) ON DELETE CASCADE,
    product_uid text NOT NULL,
    discount integer NOT NULL CHECK (discount >= 0 AND discount <= 100),
    created_at text NOT NULL DEFAULT (datetime('now')),
    UNIQUE(discount_list_id, product_uid)
);

INSERT OR IGNORE INTO discount_list_items_new (discount_list_id, product_uid, discount, created_at)
SELECT dli.discount_list_id, p.uid, dli.discount, dli.created_at
FROM discount_list_items dli
JOIN products p ON p.code = dli.product_code;

DROP TABLE discount_list_items;
ALTER TABLE discount_list_items_new RENAME TO discount_list_items;
CREATE INDEX idx_discount_list_items_product_uid ON discount_list_items(product_uid);


UPDATE price_lists SET is_active = 0;
UPDATE discount_lists SET is_active = 0;
