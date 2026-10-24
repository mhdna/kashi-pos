-- Barcode of the product the sale screen's "Unknown item" button adds. Empty:
-- the till's built-in UNKNOWN placeholder. Set from kashi's branch settings.
ALTER TABLE settings ADD COLUMN unknown_product_barcode TEXT NOT NULL DEFAULT '';
