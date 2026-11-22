-- An invoice discount that never adds up with the lines' own discounts:
-- when it applies, the lines go without their list discount and the sale
-- is charged whichever way saves more ("$50 off every $100, else 20%").
-- Central sets it (its invoice_discounts.replaces_item_discounts, 000255).
ALTER TABLE invoice_discounts ADD COLUMN replaces_item_discounts integer not null default 0;
