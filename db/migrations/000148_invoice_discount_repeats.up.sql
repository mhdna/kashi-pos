-- An amount invoice discount that comes off again for every time the sale
-- reaches its minimum: "$50 off every $100" takes $100 off a $200 sale.
-- Central sets it (its invoice_discounts.repeats, 000254).
ALTER TABLE invoice_discounts ADD COLUMN repeats integer not null default 0;
