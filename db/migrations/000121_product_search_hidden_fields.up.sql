-- Fields the product search dialog leaves out (any of code, name, brand,
-- color, size, price), comma-separated. Empty: all shown. Set from kashi's
-- branch settings.
ALTER TABLE settings ADD COLUMN product_search_hidden_fields TEXT NOT NULL DEFAULT '';
