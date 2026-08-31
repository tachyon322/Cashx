-- Pure DDL mirror of migrations/00002_accounts.sql for sqlc.
CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email            text NOT NULL UNIQUE,
    name             text NOT NULL,
    role             text NOT NULL CHECK (role IN ('partner', 'staff')),
    is_active        boolean NOT NULL DEFAULT true,
    password         varchar(255),
    email_verified_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token       varchar(255) NOT NULL UNIQUE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    last_access timestamptz NOT NULL,
    metadata    jsonb
);

CREATE TABLE verifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject    varchar(255) NOT NULL,
    value      varchar(255) NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX verifications_subject_idx ON verifications(subject);

CREATE TABLE staff_role_assignments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('superadmin', 'project_manager', 'finance', 'content_manager', 'support')),
    project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, role, project_id)
);

CREATE TABLE partner_profiles (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    referral_code           text NOT NULL UNIQUE,
    referred_by             uuid REFERENCES partner_profiles(id) ON DELETE SET NULL,
    is_approved             boolean NOT NULL DEFAULT false,
    is_blocked              boolean NOT NULL DEFAULT false,
    is_owner                boolean NOT NULL DEFAULT false,
    is_admin                boolean NOT NULL DEFAULT false,
    revshare_percent_bps    integer NOT NULL DEFAULT 4000 CHECK (revshare_percent_bps >= 0 AND revshare_percent_bps <= 10000),
    telegram_user_id        bigint,
    comment                 text,
    legacy_kazik_partner_id text UNIQUE,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
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
