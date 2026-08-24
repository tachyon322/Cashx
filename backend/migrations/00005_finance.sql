-- +goose Up
CREATE TABLE wallets (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id        uuid NOT NULL UNIQUE REFERENCES partner_profiles(id) ON DELETE CASCADE,
    available_kopecks bigint NOT NULL DEFAULT 0,
    reserved_kopecks  bigint NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wallet_ledger_entries (
    id                      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_id               uuid NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    type                    text NOT NULL CHECK (type IN ('commission', 'referral_reward', 'withdrawal', 'withdrawal_refund', 'reversal', 'manual_adjustment')),
    amount_kopecks          bigint NOT NULL,
    balance_after_kopecks   bigint NOT NULL,
    ref_conversion_event_id bigint,
    ref_withdrawal_id       uuid,
    ref_referral_reward_id  uuid,
    comment                 text,
    created_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_wallet_created_idx ON wallet_ledger_entries(wallet_id, created_at);

CREATE TABLE commission_earnings (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversion_event_id bigint NOT NULL UNIQUE,
    partner_id         uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE RESTRICT,
    offer_id           uuid NOT NULL REFERENCES offers(id) ON DELETE RESTRICT,
    rate_bps           integer NOT NULL,
    amount_kopecks     bigint NOT NULL,
    external_user_id   text NOT NULL,
    reversed_at        timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX commission_earnings_partner_created_idx ON commission_earnings(partner_id, created_at);
CREATE INDEX commission_earnings_offer_created_idx ON commission_earnings(offer_id, created_at);

CREATE TABLE referral_rewards (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    commission_earning_id uuid NOT NULL UNIQUE REFERENCES commission_earnings(id) ON DELETE RESTRICT,
    partner_referral_id   uuid NOT NULL REFERENCES partner_referrals(id) ON DELETE RESTRICT,
    referrer_partner_id   uuid NOT NULL,
    invited_partner_id    uuid NOT NULL,
    referral_rate_bps     integer NOT NULL,
    amount_kopecks        bigint NOT NULL,
    reversed_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX referral_rewards_referrer_created_idx ON referral_rewards(referrer_partner_id, created_at);

CREATE TABLE withdrawal_requests (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id     uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE RESTRICT,
    amount_kopecks bigint NOT NULL,
    method         text NOT NULL CHECK (method IN ('usdt', 'sbp')),
    requisites     text NOT NULL,
    bank           text,
    fee_kopecks    bigint NOT NULL DEFAULT 0,
    usdt_amount    numeric(18,8),
    rate           numeric(12,4),
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'paid', 'rejected', 'cancelled')),
    comment        text,
    decided_at     timestamptz,
    paid_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX withdrawal_requests_partner_created_idx ON withdrawal_requests(partner_id, created_at);
CREATE INDEX withdrawal_requests_status_idx ON withdrawal_requests(status);

CREATE TABLE payout_requisites (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    method     text NOT NULL CHECK (method IN ('usdt', 'sbp')),
    value      text NOT NULL,
    bank       text,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE payout_transfers (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    withdrawal_request_id uuid NOT NULL UNIQUE REFERENCES withdrawal_requests(id) ON DELETE RESTRICT,
    external_tx_id       text,
    amount_kopecks       bigint NOT NULL,
    fee_kopecks          bigint NOT NULL DEFAULT 0,
    transferred_at       timestamptz NOT NULL DEFAULT now(),
    created_by           uuid REFERENCES users(id) ON DELETE SET NULL,
    comment              text
);

CREATE TABLE payout_rules (
    id                     uuid PRIMARY KEY DEFAULT '00000000-0000-0000-0000-000000000001'
        CHECK (id = '00000000-0000-0000-0000-000000000001'),
    min_withdraw_kopecks   bigint NOT NULL DEFAULT 500000,
    usdt_rate              numeric(12,4) NOT NULL DEFAULT 90.0000,
    sbp_fee_flat_kopecks   bigint NOT NULL DEFAULT 0,
    sbp_fee_percent_bps    integer NOT NULL DEFAULT 0,
    updated_by             uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at             timestamptz NOT NULL DEFAULT now()
);
INSERT INTO payout_rules (id) VALUES ('00000000-0000-0000-0000-000000000001') ON CONFLICT DO NOTHING;

CREATE TABLE platform_settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO platform_settings (key, value) VALUES
    ('referral_rate_default_bps', '250'),
    ('referral_rate_max_bps', '500'),
    ('branding', '{"name":"CashX","telegram_url":null,"avatar_url":null}')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS platform_settings;
DROP TABLE IF EXISTS payout_rules;
DROP TABLE IF EXISTS payout_transfers;
DROP TABLE IF EXISTS payout_requisites;
DROP TABLE IF EXISTS withdrawal_requests;
DROP TABLE IF EXISTS referral_rewards;
DROP TABLE IF EXISTS commission_earnings;
DROP TABLE IF EXISTS wallet_ledger_entries;
DROP TABLE IF EXISTS wallets;
