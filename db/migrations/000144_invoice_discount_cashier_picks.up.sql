-- An invoice discount the cashier picks for a sale (a student discount, say)
-- instead of one that applies by itself once the total is reached. Central
-- sets it (its invoice_discounts.cashier_picks); the Rewards button lists it.
ALTER TABLE invoice_discounts ADD COLUMN cashier_picks integer not null default 0;
