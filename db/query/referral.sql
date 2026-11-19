-- name: SetClientReferralCode :exec
UPDATE clients SET referral_code = ? WHERE id = ?;

-- name: GetClientByReferralCode :one
SELECT * FROM clients WHERE referral_code = ? AND referral_code <> '' LIMIT 1;

-- name: UpsertReferralProgram :exec
INSERT INTO referral_program (id, enabled, reward_percent, min_friend_spend, reward_to, friend_percent)
VALUES (1, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    enabled = excluded.enabled, reward_percent = excluded.reward_percent,
    min_friend_spend = excluded.min_friend_spend, reward_to = excluded.reward_to,
    friend_percent = excluded.friend_percent;

-- name: GetReferralProgram :one
SELECT * FROM referral_program WHERE id = 1;

-- name: UpsertReferralReward :exec
-- used_invoice_id is the till's own and is never touched by a pull.
INSERT INTO referral_rewards (uid, owner_client_uid, percent, max_cents, status, available_from, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (uid) DO UPDATE SET
    owner_client_uid = excluded.owner_client_uid, percent = excluded.percent,
    max_cents = excluded.max_cents, status = excluded.status,
    available_from = excluded.available_from, expires_at = excluded.expires_at;

-- name: ListUsableReferralRewards :many
-- A client's rewards central has as available and no sale here used; the
-- caller checks the dates.
SELECT * FROM referral_rewards
WHERE owner_client_uid = ? AND status = 'available' AND used_invoice_id IS NULL
ORDER BY expires_at = '', expires_at, uid;

-- name: MarkReferralRewardUsed :execrows
UPDATE referral_rewards SET used_invoice_id = ?
WHERE uid = ? AND status = 'available' AND used_invoice_id IS NULL;

-- name: ReleaseReferralRewardsOfInvoice :exec
UPDATE referral_rewards SET used_invoice_id = NULL WHERE used_invoice_id = ?;

-- name: SetInvoiceReferral :exec
UPDATE invoices SET referral_code = ?, referral_reward_uid = ? WHERE id = ?;

-- name: ClientHasSale :one
-- Any sale of this client on this till, cancelled ones aside.
SELECT EXISTS (
    SELECT 1 FROM invoices i
    JOIN sales_invoices s ON s.invoice_id = i.id
    WHERE i.client_id = ? AND i.cancelled_at IS NULL
) AS has_sale;
