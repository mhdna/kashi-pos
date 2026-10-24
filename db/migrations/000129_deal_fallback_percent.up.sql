-- What the units outside a set get on a "Buy 2 get 1" deal, mirrored from
-- kashi (its 000205): 0 for nothing. See deals.go. invoice_deals keeps the
-- value the sale was made with, for exchanges.
ALTER TABLE deals ADD COLUMN fallback_percent integer not null default 0;
ALTER TABLE invoice_deals ADD COLUMN fallback_percent integer not null default 0;
