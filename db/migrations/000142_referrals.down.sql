ALTER TABLE invoices DROP COLUMN referral_reward_uid;
ALTER TABLE invoices DROP COLUMN referral_code;
DROP INDEX IF EXISTS idx_referral_rewards_owner;
DROP TABLE IF EXISTS referral_rewards;
DROP TABLE IF EXISTS referral_program;
DROP INDEX IF EXISTS idx_clients_referral_code;
ALTER TABLE clients DROP COLUMN referral_code;
