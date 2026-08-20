-- +goose Up
CREATE TABLE announcements (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title        text NOT NULL,
    body         text NOT NULL,
    audience     text NOT NULL CHECK (audience IN ('all', 'partners', 'staff', 'specific_partner')),
    is_published boolean NOT NULL DEFAULT false,
    published_at timestamptz,
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);
CREATE INDEX announcements_published_idx ON announcements(published_at);

CREATE TABLE announcement_audiences (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    announcement_id uuid NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    partner_id      uuid REFERENCES partner_profiles(id) ON DELETE CASCADE,
    UNIQUE (announcement_id, partner_id)
);

CREATE TABLE announcement_reads (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    announcement_id uuid NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    reader_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (announcement_id, reader_user_id)
);

CREATE TABLE user_notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       text NOT NULL CHECK (type IN ('commission', 'referral_reward', 'withdrawal', 'announcement')),
    title      text NOT NULL,
    body       text NOT NULL,
    metadata   jsonb,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_notifications_user_created_idx ON user_notifications(user_id, created_at);
CREATE INDEX user_notifications_unread_idx ON user_notifications(user_id) WHERE read_at IS NULL;

CREATE TABLE outbox_messages (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic           text NOT NULL,
    payload         jsonb NOT NULL,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz
);
CREATE INDEX outbox_status_next_attempt_idx ON outbox_messages(status, next_attempt_at);

CREATE TABLE audit_log (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action        text NOT NULL,
    entity_type   text NOT NULL,
    entity_id     text NOT NULL,
    changes       jsonb,
    ip            inet,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity_idx ON audit_log(entity_type, entity_id);
CREATE INDEX audit_log_created_idx ON audit_log(created_at);

-- Application role grants. Financial/event/audit rows are immutable for the app
-- role: no UPDATE/DELETE on ledger, earnings, rewards, events, clicks, audit.
GRANT USAGE ON SCHEMA public TO cashx_app;
GRANT SELECT, INSERT, UPDATE ON users, password_credentials, sessions, verification_tokens,
    staff_role_assignments, partner_profiles, partner_referrals, partner_referral_clicks,
    projects, integration_keys, project_settings, offers, offer_terms_versions,
    partner_offer_accesses, tracking_links, external_user_attributions,
    daily_partner_offer_stats, wallets, withdrawal_requests, payout_requisites,
    payout_transfers, payout_rules, platform_settings, announcements,
    announcement_audiences, announcement_reads, user_notifications, outbox_messages
    TO cashx_app;
GRANT SELECT, INSERT ON wallet_ledger_entries, commission_earnings, referral_rewards,
    incoming_events, conversion_events, tracking_clicks, audit_log TO cashx_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO cashx_app;

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS outbox_messages;
DROP TABLE IF EXISTS user_notifications;
DROP TABLE IF EXISTS announcement_reads;
DROP TABLE IF EXISTS announcement_audiences;
DROP TABLE IF EXISTS announcements;
