-- +goose Up
-- Staff referral codes were seeded as 'STAFF-' || uuid-without-dashes (37
-- chars). Rewrite them to the same short 8-char format regular partners get,
-- so the cabinet shows a human-friendly code. Codes are drawn from the same
-- unambiguous alphabet used by auth.GenerateReferralCode and checked against
-- the UNIQUE(referral_code) constraint; retries are bounded because 32^8
-- combinations make collisions effectively impossible.
-- +goose StatementBegin
DO $$
DECLARE
    p record;
    candidate text;
    attempts int;
BEGIN
    FOR p IN
        SELECT pp.id
        FROM partner_profiles pp
        JOIN users u ON u.id = pp.user_id
        WHERE u.role = 'staff'
          AND pp.referral_code ~ '^STAFF-[0-9a-f]{32}$'
    LOOP
        attempts := 0;
        LOOP
            candidate := '';
            FOR i IN 1..8 LOOP
                candidate := candidate || substr(
                    'ABCDEFGHJKLMNPQRSTUVWXYZ23456789',
                    1 + floor(random() * 32)::int,
                    1
                );
            END LOOP;
            attempts := attempts + 1;
            EXIT WHEN attempts >= 50
                 OR NOT EXISTS (
                     SELECT 1 FROM partner_profiles WHERE referral_code = candidate
                 );
        END LOOP;
        IF attempts < 50 THEN
            UPDATE partner_profiles SET referral_code = candidate WHERE id = p.id;
        END IF;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Restore the original UUID-derived staff codes.
UPDATE partner_profiles p
SET referral_code = 'STAFF-' || replace(u.id::text, '-', '')
FROM users u
WHERE p.user_id = u.id
  AND u.role = 'staff'
  AND p.referral_code <> 'STAFF-' || replace(u.id::text, '-', '');
