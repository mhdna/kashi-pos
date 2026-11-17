-- Referrals (referrals.go), from central's 000243. Every client has a code
-- central gave them. A new customer's first sale can name the code of the
-- client who sent them; central then gives that client a reward: a percent
-- off one later sale, which reaches every till here.
ALTER TABLE clients ADD COLUMN referral_code text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_clients_referral_code ON clients(referral_code) WHERE referral_code <> '';

-- Central's rules, one row. Only what the till needs: whether referrals are
-- on (the code field, the receipt line) and the reward, for the receipt.
CREATE TABLE referral_program (
    id integer PRIMARY KEY CHECK (id = 1),
    enabled integer NOT NULL DEFAULT 0,
    reward_percent integer NOT NULL DEFAULT 0,
    min_friend_spend integer NOT NULL DEFAULT 0
);

-- Rewards as central has them. status is central's (available, used,
-- cancelled). used_invoice_id is this till's own: the sale here that used
-- it, so a pull that hasn't heard of that sale yet can't make it usable
-- again. Dates are RFC3339, '' for none.
CREATE TABLE referral_rewards (
    uid text PRIMARY KEY,
    owner_client_uid text NOT NULL,
    percent integer NOT NULL,
    max_cents integer NOT NULL DEFAULT 0,
    status text NOT NULL,
    available_from text NOT NULL DEFAULT '',
    expires_at text NOT NULL DEFAULT '',
    used_invoice_id integer
);
CREATE INDEX IF NOT EXISTS idx_referral_rewards_owner ON referral_rewards(owner_client_uid);

-- The code the buyer was sent with, and the reward the sale used.
ALTER TABLE invoices ADD COLUMN referral_code text NOT NULL DEFAULT '';
ALTER TABLE invoices ADD COLUMN referral_reward_uid text NOT NULL DEFAULT '';
