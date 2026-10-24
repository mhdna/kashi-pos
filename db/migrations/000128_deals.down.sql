DROP TRIGGER IF EXISTS pricing_version_deals_ins;
DROP TRIGGER IF EXISTS pricing_version_deals_upd;
DROP TRIGGER IF EXISTS pricing_version_deals_del;
DROP TABLE IF EXISTS exchange_deal_adjustments;
ALTER TABLE invoice_products DROP COLUMN deal_discount;
ALTER TABLE invoice_products DROP COLUMN deal_uid;
DROP TABLE IF EXISTS invoice_deals;
ALTER TABLE discount_list_items DROP COLUMN deal_uid;
DROP TABLE IF EXISTS deals;
