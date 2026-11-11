-- "Pick colour/size": a per-item override of the till's variant_pick_required
-- setting, synced from central's products.variant_pick. '' follows the setting,
-- 'always' makes a scan of this item ask which colour/size, 'never' doesn't.
ALTER TABLE products ADD COLUMN variant_pick TEXT NOT NULL DEFAULT '';
