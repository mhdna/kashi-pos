ALTER TABLE invoice_products DROP COLUMN invoice_discount;
ALTER TABLE invoices DROP COLUMN invoice_discount_name;
ALTER TABLE invoices DROP COLUMN invoice_discount;
DROP TABLE IF EXISTS invoice_discounts;
