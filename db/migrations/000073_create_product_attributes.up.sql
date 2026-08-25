-- Mirrors kashi's attribute system exactly (same table shapes and same
-- seeded attribute ids/names — see 000005_create_products_attributes_table.up.sql
-- in the kashi repo) so a product's brand/category/etc. can sync between
-- the two systems without a lossy field-name mapping.
--
-- This is additive only: products.brand/family/kind/season/product_year
-- stay in place, unread by the main app going forward, but still readable
-- by admin_server.go and tools/kashi-admin — which are being retired, not
-- converted, so leaving their raw-SQL paths alone rather than rewriting
-- them for a system on its way out.
CREATE TABLE IF NOT EXISTS attributes (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS attributes_values (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  attribute_id INTEGER NOT NULL REFERENCES attributes(id),
  value TEXT NOT NULL,
  UNIQUE (attribute_id, value)
);

CREATE TABLE IF NOT EXISTS products_attributes (
  attribute_id INTEGER NOT NULL REFERENCES attributes(id),
  attribute_value_id INTEGER NOT NULL REFERENCES attributes_values(id),
  product_id INTEGER NOT NULL REFERENCES products(id),
  PRIMARY KEY (attribute_id, product_id)
);

INSERT INTO attributes (id, name) VALUES
  (0, 'category'),
  (1, 'sub-category'),
  (2, 'brand'),
  (3, 'kind'),
  (4, 'type'),
  (5, 'unit'),
  (6, 'year'),
  (7, 'season'),
  (8, 'origin');

-- Backfill existing free-text values. kashi-pos's own "family" column has
-- no like-named counterpart in kashi's attribute set — mapped to
-- 'sub-category' as the closest existing concept (category > sub-category
-- reads as the same broad-to-narrow hierarchy "family" was used for here).
INSERT INTO attributes_values (attribute_id, value)
  SELECT DISTINCT 2, brand FROM products WHERE brand != '';
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
  SELECT 2, av.id, p.id FROM products p
  JOIN attributes_values av ON av.attribute_id = 2 AND av.value = p.brand
  WHERE p.brand != '';

INSERT INTO attributes_values (attribute_id, value)
  SELECT DISTINCT 1, family FROM products WHERE family != '';
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
  SELECT 1, av.id, p.id FROM products p
  JOIN attributes_values av ON av.attribute_id = 1 AND av.value = p.family
  WHERE p.family != '';

INSERT INTO attributes_values (attribute_id, value)
  SELECT DISTINCT 3, kind FROM products WHERE kind != '';
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
  SELECT 3, av.id, p.id FROM products p
  JOIN attributes_values av ON av.attribute_id = 3 AND av.value = p.kind
  WHERE p.kind != '';

INSERT INTO attributes_values (attribute_id, value)
  SELECT DISTINCT 7, season FROM products WHERE season != '';
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
  SELECT 7, av.id, p.id FROM products p
  JOIN attributes_values av ON av.attribute_id = 7 AND av.value = p.season
  WHERE p.season != '';

INSERT INTO attributes_values (attribute_id, value)
  SELECT DISTINCT 6, CAST(product_year AS TEXT) FROM products WHERE product_year != 0;
INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
  SELECT 6, av.id, p.id FROM products p
  JOIN attributes_values av ON av.attribute_id = 6 AND av.value = CAST(p.product_year AS TEXT)
  WHERE p.product_year != 0;
