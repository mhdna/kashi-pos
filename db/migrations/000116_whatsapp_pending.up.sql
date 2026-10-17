-- Post-sale WhatsApp thank-yous kashi hasn't confirmed yet. Saved before the
-- request so one cut off (the app closing for an update, no internet) is sent
-- again later; removed once kashi answers (whatsapp.go).
CREATE TABLE IF NOT EXISTS whatsapp_pending (
    invoice_code TEXT PRIMARY KEY,
    payload TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    attempts INTEGER NOT NULL DEFAULT 0
);
