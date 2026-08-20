-- name: CreateWallet :one
INSERT INTO wallets (partner_id) VALUES ($1) RETURNING id, partner_id, available_kopecks, reserved_kopecks, created_at, updated_at;

-- name: GetWalletByPartnerID :one
SELECT id, partner_id, available_kopecks, reserved_kopecks, created_at, updated_at FROM wallets WHERE partner_id = $1;

-- name: GetWalletByID :one
SELECT id, partner_id, available_kopecks, reserved_kopecks, created_at, updated_at FROM wallets WHERE id = $1;

-- name: CreditWallet :one
UPDATE wallets
SET available_kopecks = available_kopecks + $2, updated_at = now()
WHERE id = $1
RETURNING id, partner_id, available_kopecks, reserved_kopecks, updated_at;

-- name: ReserveWithdrawal :one
UPDATE wallets
SET available_kopecks = available_kopecks - $2, reserved_kopecks = reserved_kopecks + $2, updated_at = now()
WHERE id = $1 AND available_kopecks >= $2
RETURNING id, partner_id, available_kopecks, reserved_kopecks, updated_at;

-- name: ReleaseReserve :one
UPDATE wallets
SET available_kopecks = available_kopecks + $2, reserved_kopecks = reserved_kopecks - $2, updated_at = now()
WHERE id = $1 AND reserved_kopecks >= $2
RETURNING id, partner_id, available_kopecks, reserved_kopecks, updated_at;

-- name: ConsumeReserve :one
UPDATE wallets
SET reserved_kopecks = reserved_kopecks - $2, updated_at = now()
WHERE id = $1 AND reserved_kopecks >= $2
RETURNING id, partner_id, available_kopecks, reserved_kopecks, updated_at;

-- name: InsertLedgerEntry :one
INSERT INTO wallet_ledger_entries (wallet_id, type, amount_kopecks, balance_after_kopecks, ref_conversion_event_id, ref_withdrawal_id, ref_referral_reward_id, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, wallet_id, type, amount_kopecks, balance_after_kopecks, created_at;

-- name: ListLedgerByWallet :many
SELECT id, wallet_id, type, amount_kopecks, balance_after_kopecks, ref_conversion_event_id, ref_withdrawal_id, ref_referral_reward_id, comment, created_at
FROM wallet_ledger_entries WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListLedgerAdmin :many
SELECT l.id, l.wallet_id, l.type, l.amount_kopecks, l.balance_after_kopecks, l.comment, l.created_at,
       w.partner_id, u.name AS partner_name
FROM wallet_ledger_entries l
JOIN wallets w ON w.id = l.wallet_id
JOIN partner_profiles p ON p.id = w.partner_id
JOIN users u ON u.id = p.user_id
WHERE ($1 = '' OR w.partner_id = $1::uuid)
  AND ($2::timestamptz IS NULL OR l.created_at >= $2)
  AND ($3::timestamptz IS NULL OR l.created_at <= $3)
ORDER BY l.created_at DESC LIMIT $4 OFFSET $5;

-- name: CountLedgerAdmin :one
SELECT count(*) FROM wallet_ledger_entries l
JOIN wallets w ON w.id = l.wallet_id
WHERE ($1 = '' OR w.partner_id = $1::uuid)
  AND ($2::timestamptz IS NULL OR l.created_at >= $2)
  AND ($3::timestamptz IS NULL OR l.created_at <= $3);

-- name: InsertCommissionEarning :one
INSERT INTO commission_earnings (conversion_event_id, partner_id, offer_id, tracking_link_id, rate_bps, amount_kopecks, external_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, conversion_event_id, partner_id, offer_id, tracking_link_id, rate_bps, amount_kopecks, external_user_id, reversed_at, created_at;

-- name: GetEarningByConversion :one
SELECT id, conversion_event_id, partner_id, offer_id, tracking_link_id, rate_bps, amount_kopecks, external_user_id, reversed_at, created_at
FROM commission_earnings WHERE conversion_event_id = $1;

-- name: ReverseEarning :one
UPDATE commission_earnings SET reversed_at = now()
WHERE id = $1 AND reversed_at IS NULL
RETURNING id, conversion_event_id, partner_id, offer_id, tracking_link_id, rate_bps, amount_kopecks, external_user_id, reversed_at, created_at;

-- name: ListEarningsAdmin :many
SELECT e.id, e.conversion_event_id, e.partner_id, e.offer_id, e.tracking_link_id, e.rate_bps, e.amount_kopecks, e.external_user_id, e.reversed_at, e.created_at
FROM commission_earnings e
WHERE ($1 = '' OR e.partner_id = $1::uuid)
  AND ($2 = '' OR e.offer_id = $2::uuid)
  AND ($3::timestamptz IS NULL OR e.created_at >= $3)
  AND ($4::timestamptz IS NULL OR e.created_at <= $4)
ORDER BY e.created_at DESC LIMIT $5 OFFSET $6;

-- name: CountEarningsAdmin :one
SELECT count(*) FROM commission_earnings e
WHERE ($1 = '' OR e.partner_id = $1::uuid)
  AND ($2 = '' OR e.offer_id = $2::uuid)
  AND ($3::timestamptz IS NULL OR e.created_at >= $3)
  AND ($4::timestamptz IS NULL OR e.created_at <= $4);

-- name: InsertReferralReward :one
INSERT INTO referral_rewards (commission_earning_id, partner_referral_id, referrer_partner_id, invited_partner_id, referral_rate_bps, amount_kopecks)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, commission_earning_id, partner_referral_id, referrer_partner_id, invited_partner_id, referral_rate_bps, amount_kopecks, reversed_at, created_at;

-- name: GetRewardByEarning :one
SELECT id, commission_earning_id, partner_referral_id, referrer_partner_id, invited_partner_id, referral_rate_bps, amount_kopecks, reversed_at, created_at
FROM referral_rewards WHERE commission_earning_id = $1;

-- name: ReverseReward :one
UPDATE referral_rewards SET reversed_at = now()
WHERE id = $1 AND reversed_at IS NULL
RETURNING id, commission_earning_id, partner_referral_id, referrer_partner_id, invited_partner_id, referral_rate_bps, amount_kopecks, reversed_at, created_at;

-- name: SumRewardsByReferrer :one
SELECT COALESCE(sum(amount_kopecks), 0)::bigint FROM referral_rewards WHERE referrer_partner_id = $1 AND reversed_at IS NULL;

-- name: SumRewardsByInvited :one
SELECT COALESCE(sum(amount_kopecks), 0)::bigint FROM referral_rewards WHERE invited_partner_id = $1 AND reversed_at IS NULL;
