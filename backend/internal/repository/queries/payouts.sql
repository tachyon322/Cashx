-- name: GetPayoutRules :one
SELECT id, min_withdraw_kopecks, usdt_rate, sbp_fee_flat_kopecks, sbp_fee_percent_bps, updated_by, updated_at
FROM payout_rules WHERE id = '00000000-0000-0000-0000-000000000001';

-- name: UpdatePayoutRules :one
UPDATE payout_rules
SET min_withdraw_kopecks = $1, usdt_rate = $2, sbp_fee_flat_kopecks = $3, sbp_fee_percent_bps = $4,
    updated_by = $5, updated_at = now()
WHERE id = '00000000-0000-0000-0000-000000000001'
RETURNING id, min_withdraw_kopecks, usdt_rate, sbp_fee_flat_kopecks, sbp_fee_percent_bps, updated_by, updated_at;

-- name: InsertWithdrawalRequest :one
INSERT INTO withdrawal_requests (partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate, status, comment, decided_at, paid_at, created_at, updated_at;

-- name: GetWithdrawalRequest :one
SELECT id, partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate, status, comment, decided_at, paid_at, created_at, updated_at
FROM withdrawal_requests WHERE id = $1;

-- name: DecideWithdrawal :one
UPDATE withdrawal_requests
SET status = $2, comment = COALESCE($3, comment), decided_at = now(), updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING id, partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate, status, comment, decided_at, paid_at, created_at, updated_at;

-- name: MarkWithdrawalPaid :one
UPDATE withdrawal_requests
SET status = 'paid', comment = COALESCE($2, comment), paid_at = now(), updated_at = now()
WHERE id = $1 AND status = 'approved'
RETURNING id, partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate, status, comment, decided_at, paid_at, created_at, updated_at;

-- name: ListWithdrawalsByPartner :many
SELECT id, partner_id, amount_kopecks, method, requisites, bank, fee_kopecks, usdt_amount, rate, status, comment, decided_at, paid_at, created_at, updated_at
FROM withdrawal_requests WHERE partner_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListWithdrawalsAdmin :many
SELECT w.id, w.partner_id, w.amount_kopecks, w.method, w.requisites, w.bank, w.fee_kopecks, w.usdt_amount, w.rate, w.status, w.comment, w.decided_at, w.paid_at, w.created_at, w.updated_at,
       u.name AS partner_name, u.email AS partner_email
FROM withdrawal_requests w
JOIN partner_profiles p ON p.id = w.partner_id
JOIN users u ON u.id = p.user_id
WHERE ($1 = '' OR w.status = $1)
  AND ($2 = '' OR w.partner_id = $2::uuid)
ORDER BY w.created_at DESC LIMIT $3 OFFSET $4;

-- name: CountWithdrawalsAdmin :one
SELECT count(*) FROM withdrawal_requests w
WHERE ($1 = '' OR w.status = $1)
  AND ($2 = '' OR w.partner_id = $2::uuid);

-- name: InsertPayoutTransfer :one
INSERT INTO payout_transfers (withdrawal_request_id, external_tx_id, amount_kopecks, fee_kopecks, created_by, comment)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, withdrawal_request_id, external_tx_id, amount_kopecks, fee_kopecks, transferred_at, created_by, comment;
