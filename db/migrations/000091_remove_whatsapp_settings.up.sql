-- WhatsApp (Meta's WhatsApp Business Platform / Cloud API) config and
-- sending now live centrally on kashi -- see api/whatsapp.go in that repo
-- and POST /branch/whatsapp/thank_you. kashi-pos no longer has any
-- WhatsApp-related settings; it just reports the sale's details.
ALTER TABLE settings DROP COLUMN whatsapp_enabled;
ALTER TABLE settings DROP COLUMN whatsapp_api_url;
ALTER TABLE settings DROP COLUMN whatsapp_api_key;
ALTER TABLE settings DROP COLUMN whatsapp_session;
ALTER TABLE settings DROP COLUMN whatsapp_message;
ALTER TABLE settings DROP COLUMN whatsapp_review_url;
ALTER TABLE settings DROP COLUMN whatsapp_phone_number_id;
ALTER TABLE settings DROP COLUMN whatsapp_access_token;
ALTER TABLE settings DROP COLUMN whatsapp_template_name;
ALTER TABLE settings DROP COLUMN whatsapp_template_lang;
ALTER TABLE settings DROP COLUMN whatsapp_template_params;
