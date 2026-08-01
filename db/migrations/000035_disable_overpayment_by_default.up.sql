-- Overpayment is opt-in. Existing installations receive the new safe default
-- and can enable it again from Settings.
UPDATE settings SET max_overpayment_cents = 0;
