-- Invoice payment summaries may be negative when an account is used to give
-- change. Zero-net accounts remain represented by their detailed entries but
-- do not need a summary row.
CREATE TABLE invoice_payments_new (
    invoice_id bigint NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    amount bigint NOT NULL CHECK (amount <> 0),
    PRIMARY KEY (invoice_id, cashbox_id)
);

INSERT INTO invoice_payments_new (invoice_id, cashbox_id, amount)
SELECT invoice_id, cashbox_id, amount FROM invoice_payments;

DROP TABLE invoice_payments;
ALTER TABLE invoice_payments_new RENAME TO invoice_payments;

ALTER TABLE settings ADD COLUMN market_name text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN market_phone text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN market_description text NOT NULL DEFAULT 'Shop for all';
ALTER TABLE settings ADD COLUMN return_policy text NOT NULL DEFAULT 'Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt.';
ALTER TABLE settings ADD COLUMN website text NOT NULL DEFAULT 'yourwebsite.com';
ALTER TABLE settings ADD COLUMN instagram text NOT NULL DEFAULT '@your_instagram';
ALTER TABLE settings ADD COLUMN receipt_font text NOT NULL DEFAULT 'Belleza';
ALTER TABLE settings ADD COLUMN receipt_title_size real NOT NULL DEFAULT 21;
ALTER TABLE settings ADD COLUMN receipt_body_size real NOT NULL DEFAULT 11;
ALTER TABLE settings ADD COLUMN receipt_cutoff integer NOT NULL DEFAULT 1 CHECK (receipt_cutoff IN (0, 1));
ALTER TABLE settings ADD COLUMN receipt_body_font text NOT NULL DEFAULT 'Printer Font A';
ALTER TABLE settings ADD COLUMN branch_phone text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN counter_number text NOT NULL DEFAULT '1';
