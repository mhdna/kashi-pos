-- Tills that were on 1.1.148 or older when central gave every client a
-- referral code (central's 000247) pulled those rows before they knew about
-- codes: they stamped the new versions and moved their clients cursor past
-- them, so older clients never got a code here and their receipts (and
-- reprints) printed none. Forget the version of every client still without
-- one and the clients cursor: the next pull checks every client against
-- central (fullCheckEntity) and fetches only these again.
UPDATE clients SET remote_updated_at = NULL
WHERE referral_code = '' AND uid IS NOT NULL AND uid <> '';
DELETE FROM sync_cursors WHERE entity = 'clients';
