-- +goose Up
-- Staff users also have their own partner identity so they can use the cabinet.
INSERT INTO partner_profiles (user_id, referral_code, is_approved, is_blocked)
SELECT u.id, 'STAFF-' || replace(u.id::text, '-', ''), true, false
FROM users u
WHERE u.role = 'staff'
  AND NOT EXISTS (
      SELECT 1 FROM partner_profiles p WHERE p.user_id = u.id
  )
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO wallets (partner_id)
SELECT p.id
FROM partner_profiles p
JOIN users u ON u.id = p.user_id
WHERE u.role = 'staff'
ON CONFLICT (partner_id) DO NOTHING;

-- +goose Down
DELETE FROM partner_profiles p
USING users u
WHERE p.user_id = u.id
  AND u.role = 'staff'
  AND p.referral_code = 'STAFF-' || replace(u.id::text, '-', '');
