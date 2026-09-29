-- Soft-delete flag for clients, mirroring products.is_active. Sync All
-- deactivates local clients whose phone is no longer known to kashi instead
-- of hard-deleting them, so past invoices/shift reports still resolve.
ALTER TABLE clients ADD COLUMN is_active integer NOT NULL DEFAULT 1;
