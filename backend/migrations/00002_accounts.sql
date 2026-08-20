-- +goose Up
CREATE TABLE users (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       text NOT NULL UNIQUE,
    name        text NOT NULL,
    role        text NOT NULL CHECK (role IN ('partner', 'staff')),
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE password_credentials (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   text NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    ip           inet,
    user_agent   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);

CREATE TABLE verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('email_verification', 'password_reset')),
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX verification_tokens_user_id_idx ON verification_tokens(user_id);

-- FK to projects is added in 00003 (projects created there).
CREATE TABLE staff_role_assignments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('superadmin', 'project_manager', 'finance', 'content_manager', 'support')),
    project_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, role, project_id)
);

CREATE TABLE partner_profiles (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    referral_code    text NOT NULL UNIQUE,
    referred_by      uuid REFERENCES partner_profiles(id) ON DELETE SET NULL,
    is_approved      boolean NOT NULL DEFAULT false,
    is_blocked       boolean NOT NULL DEFAULT false,
    telegram_user_id bigint,
    comment          text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE partner_referrals (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_partner_id  uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    invited_partner_id   uuid NOT NULL UNIQUE REFERENCES partner_profiles(id) ON DELETE CASCADE,
    referral_rate_bps    integer NOT NULL DEFAULT 250,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX partner_referrals_referrer_idx ON partner_referrals(referrer_partner_id);

CREATE TABLE partner_referral_clicks (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    referrer_partner_id uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    ip                  inet,
    user_agent          text,
    referrer            text,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX partner_referral_clicks_referrer_created_idx ON partner_referral_clicks(referrer_partner_id, created_at);

CREATE TABLE media_assets (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket       text NOT NULL,
    key          text NOT NULL UNIQUE,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL,
    uploaded_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS media_assets;
DROP TABLE IF EXISTS partner_referral_clicks;
DROP TABLE IF EXISTS partner_referrals;
DROP TABLE IF EXISTS partner_profiles;
DROP TABLE IF EXISTS staff_role_assignments;
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS password_credentials;
DROP TABLE IF EXISTS users;
