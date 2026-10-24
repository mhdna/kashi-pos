-- Mirrored from kashi (its 000207): the deal's fallback applies only when the
-- cart makes no set, not to the units a full set leaves over. invoice_deals
-- keeps the value the sale was made with, for exchanges. See deals.go.
ALTER TABLE deals ADD COLUMN fallback_only_without_set integer not null default 0;
ALTER TABLE invoice_deals ADD COLUMN fallback_only_without_set integer not null default 0;
