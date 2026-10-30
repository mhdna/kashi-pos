-- What a unit on a deal gets outside a set, in place of the deal's fallback
-- (kashi's 000216): one "Buy 2 get 1" pool for the whole shop, the outlet
-- codes' leftover units at 30% and the rest at 20%. NULL is the deal's own.
ALTER TABLE discount_list_items ADD COLUMN deal_fallback_percent integer;

-- The same on a sold line, so an exchange judges the sale by the fallback it
-- was sold with, as it does by the deal's terms (invoice_deals).
ALTER TABLE invoice_products ADD COLUMN deal_fallback_percent integer;
