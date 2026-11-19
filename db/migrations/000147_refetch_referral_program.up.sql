-- Tills that pulled the referral program before they knew who it rewards
-- (000146) kept reward_to 'owner' and their cursor past it, so a program
-- set to reward both (or the new customer) still printed the owner's note
-- and gave the new customer nothing. Forget the cursor: the next pull
-- fetches the program again.
DELETE FROM sync_cursors WHERE entity = 'referral_program';
