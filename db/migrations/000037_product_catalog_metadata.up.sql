ALTER TABLE products ADD COLUMN brand text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN family text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN kind text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN season text NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN product_year integer NOT NULL DEFAULT 0 CHECK (product_year >= 0);

CREATE INDEX IF NOT EXISTS idx_products_variant
    ON products(code, color, size);
