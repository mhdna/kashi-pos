-- The name the cashier gave an "Unknown item" when ringing it up, so the invoice
-- list and the receipt show it instead of "Unknown item". '' for every other line.
ALTER TABLE invoice_products ADD COLUMN item_name TEXT NOT NULL DEFAULT '';
