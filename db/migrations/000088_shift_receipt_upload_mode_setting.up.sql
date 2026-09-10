-- How the close-shift receipt photo is collected. An enum, so it can't ride
-- in feature_flags (that column is CHECK (enabled IN (0,1))):
--   0 off | 1 central (QR -> kashi) | 2 hybrid (kashi when online, else this
--   POS) | 3 local (QR -> this POS's own upload server, always).
-- Default 2 = hybrid. kashi can change it per branch via the update_settings
-- command (see executeUpdateSettingsCommand).
ALTER TABLE settings ADD COLUMN shift_receipt_upload_mode integer NOT NULL DEFAULT 2;
