-- name: GetUserByID :one
SELECT id, email, name, role, is_active, created_at, updated_at FROM users WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, email, name, role, is_active, created_at, updated_at;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: CreateStaffRoleAssignment :exec
INSERT INTO staff_role_assignments (user_id, role, project_id) VALUES ($1, $2, $3);

-- name: GetStaffRolesByUserID :many
SELECT role, project_id FROM staff_role_assignments WHERE user_id = $1;

-- name: CreatePartnerProfile :one
INSERT INTO partner_profiles (user_id, referral_code, referred_by)
VALUES ($1, $2, $3) RETURNING id, user_id, referral_code, referred_by, is_approved, is_blocked, revshare_percent_bps, telegram_user_id, comment, created_at, updated_at;

-- name: GetPartnerProfileByUserID :one
SELECT id, user_id, referral_code, referred_by, is_approved, is_blocked, revshare_percent_bps, telegram_user_id, comment, created_at, updated_at
FROM partner_profiles WHERE user_id = $1;

-- name: GetPartnerProfileByID :one
SELECT id, user_id, referral_code, referred_by, is_approved, is_blocked, revshare_percent_bps, telegram_user_id, comment, created_at, updated_at
FROM partner_profiles WHERE id = $1;

-- name: GetPartnerProfileByReferralCode :one
SELECT id, user_id, referral_code, referred_by, is_approved, is_blocked, revshare_percent_bps, telegram_user_id, comment, created_at, updated_at
FROM partner_profiles WHERE referral_code = $1;

-- name: UpdatePartnerProfile :one
UPDATE partner_profiles
SET is_approved = COALESCE(sqlc.narg('is_approved'), is_approved),
    is_blocked = COALESCE(sqlc.narg('is_blocked'), is_blocked),
    revshare_percent_bps = COALESCE(sqlc.narg('revshare_percent_bps'), revshare_percent_bps),
    telegram_user_id = COALESCE(sqlc.narg('telegram_user_id'), telegram_user_id),
    comment = COALESCE(sqlc.narg('comment'), comment),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, user_id, referral_code, referred_by, is_approved, is_blocked, revshare_percent_bps, telegram_user_id, comment, created_at, updated_at;

-- name: SetPartnerRevShare :exec
UPDATE partner_profiles SET revshare_percent_bps = $2, updated_at = now() WHERE id = $1;

-- name: GetUserByPartnerID :one
SELECT u.id, u.email, u.name, u.role, u.is_active, u.created_at, u.updated_at
FROM users u JOIN partner_profiles p ON p.user_id = u.id WHERE p.id = $1;

-- name: GetUserIDByPartnerID :one
SELECT user_id FROM partner_profiles WHERE id = $1;

-- name: ListPartnerProfilesAdmin :many
SELECT p.id, p.user_id, p.referral_code, p.is_approved, p.is_blocked, p.telegram_user_id, p.comment, p.created_at,
       u.email, u.name,
       w.available_kopecks, w.reserved_kopecks
FROM partner_profiles p
JOIN users u ON u.id = p.user_id
JOIN wallets w ON w.partner_id = p.id
WHERE ($1 = '' OR u.name ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%')
  AND ($2 = 'pending' OR $2 = '' OR ($2 = 'blocked' AND p.is_blocked) OR ($2 = 'active' AND NOT p.is_blocked AND p.is_approved))
ORDER BY p.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountPartnerProfilesAdmin :one
SELECT count(*) FROM partner_profiles p
JOIN users u ON u.id = p.user_id
WHERE ($1 = '' OR u.name ILIKE '%' || $1 || '%' OR u.email ILIKE '%' || $1 || '%')
  AND ($2 = 'pending' OR $2 = '' OR ($2 = 'blocked' AND p.is_blocked) OR ($2 = 'active' AND NOT p.is_blocked AND p.is_approved));

-- name: ListAccessRatesAll :many
SELECT a.id, a.partner_id, a.offer_id, a.rate_bps, a.status
FROM partner_offer_accesses a
ORDER BY a.created_at;

-- name: ListReferralsByReferrer :many
SELECT r.id, r.referrer_partner_id, r.invited_partner_id, r.referral_rate_bps, r.created_at,
       u.name, u.email
FROM partner_referrals r
JOIN partner_profiles p ON p.id = r.invited_partner_id
JOIN users u ON u.id = p.user_id
WHERE r.referrer_partner_id = $1
ORDER BY r.created_at DESC;

-- name: CountReferralsByReferrer :one
SELECT count(*) FROM partner_referrals WHERE referrer_partner_id = $1;

-- name: GetPartnerReferralByInvited :one
SELECT id, referrer_partner_id, invited_partner_id, referral_rate_bps, created_at
FROM partner_referrals WHERE invited_partner_id = $1;

-- name: CreatePartnerReferral :one
INSERT INTO partner_referrals (referrer_partner_id, invited_partner_id, referral_rate_bps)
VALUES ($1, $2, $3) RETURNING id, referrer_partner_id, invited_partner_id, referral_rate_bps, created_at;

-- name: CreatePartnerReferralClick :exec
INSERT INTO partner_referral_clicks (referrer_partner_id, ip, user_agent, referrer)
VALUES ($1, $2::inet, $3, $4);


-- name: UpdateUserPassword :exec
UPDATE users SET password = $2, updated_at = now() WHERE id = $1;
