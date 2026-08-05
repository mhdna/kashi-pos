ALTER TABLE coupon_types ADD COLUMN reference_only INTEGER NOT NULL DEFAULT 0;

CREATE TABLE invoice_coupon_payments_new (
    invoice_id bigint PRIMARY KEY REFERENCES invoices(id) ON DELETE CASCADE,
    coupon_code text NOT NULL,
    coupon_type_id bigint NOT NULL REFERENCES coupon_types(id),
    amount bigint NOT NULL CHECK (amount > 0)
);

INSERT INTO invoice_coupon_payments_new (invoice_id, coupon_code, coupon_type_id, amount)
SELECT invoice_id, coupon_code, coupon_type_id, amount
FROM invoice_coupon_payments;

DROP TABLE invoice_coupon_payments;
ALTER TABLE invoice_coupon_payments_new RENAME TO invoice_coupon_payments;
