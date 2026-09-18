-- Switch the post-sale WhatsApp "thank you / please rate us" message from the
-- open-wa bridge to Meta's official WhatsApp Business Platform (Cloud API).
-- Cloud API requires a pre-approved message template for business-initiated
-- messages, so the free-form {placeholder} template is replaced by a
-- template name + an ordered list of body parameters.
--   whatsapp_phone_number_id   Cloud API phone number ID (the sender)
--   whatsapp_access_token      System user / temporary access token, sent as
--                              `Authorization: Bearer <token>`
--   whatsapp_template_name     Approved template name, e.g. order_confirmation
--   whatsapp_template_lang     Template language code, e.g. en_US
--   whatsapp_template_params   Comma-separated list of placeholders that fill
--                              the template's {{1}}, {{2}}, ... body
--                              parameters in order. Supported placeholders:
--                              name, market, branch, invoice, total
ALTER TABLE settings ADD COLUMN whatsapp_phone_number_id text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_access_token text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_template_name text NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN whatsapp_template_lang text NOT NULL DEFAULT 'en_US';
ALTER TABLE settings ADD COLUMN whatsapp_template_params text NOT NULL DEFAULT 'name,invoice,total';
