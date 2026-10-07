-- Display preferences the till's screen used to keep only in localStorage
-- (quick access rows, card text size, which card lines show, animations,
-- calculator's leading currency, theme). They live here too now so kashi
-- can see them in the settings report and push new values with an
-- update_settings command (see display_prefs.go).
--
-- display_prefs is a JSON object of whichever prefs are known; the screen
-- writes its current values here on every local change. display_prefs_rev
-- only moves when kashi pushes values: the screen remembers the last rev it
-- applied and adopts the stored prefs whenever this one is newer.
ALTER TABLE settings ADD COLUMN display_prefs text NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN display_prefs_rev integer NOT NULL DEFAULT 0;
