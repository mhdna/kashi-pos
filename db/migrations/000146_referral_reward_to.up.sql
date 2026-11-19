-- Who a referral code rewards (central's 000253): 'owner' (the client who
-- shared it, a later sale, as before), 'friend' (the new customer, off the
-- sale they use it on, here at the till) or 'both'; friend_percent is the
-- new customer's percent.
ALTER TABLE referral_program ADD COLUMN reward_to text NOT NULL DEFAULT 'owner';
ALTER TABLE referral_program ADD COLUMN friend_percent integer NOT NULL DEFAULT 0;
