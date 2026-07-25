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
