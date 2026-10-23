-- A counter every change to what prices a sale bumps, so the till can keep
-- the active prices and discounts in memory (pricing_overrides.go) and still
-- never quote a stale one: before reusing them it reads this row, and any
-- write to these tables -- the sync, an admin edit, anything added later --
-- has moved it. token tells one database's counter from another's.
CREATE TABLE pricing_version (
    id integer primary key check (id = 1),
    token text not null,
    version integer not null default 0
);
INSERT INTO pricing_version (id, token) VALUES (1, lower(hex(randomblob(8))));

CREATE TRIGGER pricing_version_price_lists_ins AFTER INSERT ON price_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_price_lists_upd AFTER UPDATE ON price_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_price_lists_del AFTER DELETE ON price_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_price_list_items_ins AFTER INSERT ON price_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_price_list_items_upd AFTER UPDATE ON price_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_price_list_items_del AFTER DELETE ON price_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_lists_ins AFTER INSERT ON discount_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_lists_upd AFTER UPDATE ON discount_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_lists_del AFTER DELETE ON discount_lists BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_items_ins AFTER INSERT ON discount_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_items_upd AFTER UPDATE ON discount_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_items_del AFTER DELETE ON discount_list_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_option_items_ins AFTER INSERT ON discount_list_option_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_option_items_upd AFTER UPDATE ON discount_list_option_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_discount_list_option_items_del AFTER DELETE ON discount_list_option_items BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_product_sale_options_ins AFTER INSERT ON product_sale_options BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_product_sale_options_upd AFTER UPDATE ON product_sale_options BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_product_sale_options_del AFTER DELETE ON product_sale_options BEGIN UPDATE pricing_version SET version = version + 1; END;
-- Sale options are keyed by uid in memory (ListAllProductSaleOptions).
CREATE TRIGGER pricing_version_products_uid AFTER UPDATE OF uid ON products BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_products_del AFTER DELETE ON products BEGIN UPDATE pricing_version SET version = version + 1; END;
