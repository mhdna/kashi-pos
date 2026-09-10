-- Post-sale WhatsApp "thank you / please rate us" message.
-- The POS talks to an open-wa (@open-wa/wa-automate) EASY API server, which
-- exposes a plain HTTP endpoint we POST the outgoing text to. All fields live
-- on settings so kashi can push them per branch via the update_settings command
-- (see executeUpdateSettingsCommand).
--   whatsapp_enabled          0 off | 1 on
--   whatsapp_api_url          base URL of the open-wa EASY API, e.g. http://localhost:8002
--   whatsapp_api_key          value sent as the `api_key` header (open-wa --key)
--   whatsapp_session          open-wa session name, path segment on the EASY API ("" -> "session")
--   whatsapp_message          template; placeholders:
--                               {name} {market} {branch} {invoice} {total} {items} {link}
--                             {items} expands to bullet lines of the purchased
--                             products; {link} / {items} collapse when empty.
--   whatsapp_review_url       substituted for {link}; blank drops the placeholder
-- The message is sent as an image whose picture is a QR code of {invoice},
-- with the text below as the caption (see whatsapp.go).
ALTER TABLE settings ADD COLUMN whatsapp_enabled integer NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN whatsapp_api_url text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_api_key text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_session text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_message text NOT NULL DEFAULT 'Hi {name}, thank you for shopping with {market}!

Your order {invoice}:
{items}
Total: {total}

We''d love to hear how we did - please rate your experience with us: {link}';
ALTER TABLE settings ADD COLUMN whatsapp_review_url text NOT NULL DEFAULT '';
