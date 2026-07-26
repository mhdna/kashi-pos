-- Keep lookup performance intact for databases created before these indexes
-- were guaranteed by the migration history.
CREATE INDEX IF NOT EXISTS idx_clients_phone ON clients(phone);
CREATE INDEX IF NOT EXISTS idx_products_barcode ON products(barcode);
