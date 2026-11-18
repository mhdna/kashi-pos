-- The printed invoice's own wording and which parts it leaves out, as JSON
-- (receipt_texts.go); '' is every default. Kashi's Receipt Designer edits it
-- and sends it with update_settings.
ALTER TABLE settings ADD COLUMN receipt_texts text NOT NULL DEFAULT '';
