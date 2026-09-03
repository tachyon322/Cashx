-- Pure DDL mirror of migrations/00004_tracking.sql for sqlc (no triggers, no seeds).
CREATE TABLE tracking_clicks (
    id                bigint GENERATED ALWAYS AS IDENTITY,
    tracking_link_id  uuid NOT NULL REFERENCES tracking_links(id) ON DELETE RESTRICT,
    ip                inet,
    user_agent        text,
    referrer          text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX tracking_clicks_link_created_idx ON tracking_clicks(tracking_link_id, created_at);
CREATE INDEX tracking_clicks_created_idx ON tracking_clicks(created_at);

CREATE TABLE external_user_attributions (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id         uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tracking_click_id  bigint,
    tracking_link_id   uuid,
    partner_id         uuid,
    offer_id           uuid,
    external_user_id   text NOT NULL,
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, external_user_id)
);
CREATE INDEX attributions_partner_offer_firstseen_idx
    ON external_user_attributions(partner_id, offer_id, first_seen_at);
CREATE INDEX attributions_link_firstseen_idx
    ON external_user_attributions(tracking_link_id, first_seen_at);

CREATE TABLE incoming_events (
    id                bigint GENERATED ALWAYS AS IDENTITY,
    project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    external_event_id text NOT NULL,
    type              text NOT NULL CHECK (type IN ('registration.created', 'revenue.confirmed', 'revenue.reversed')),
    payload           jsonb NOT NULL,
    status            text NOT NULL CHECK (status IN ('processed', 'ignored')),
    reason            text,
    received_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, received_at)
) PARTITION BY RANGE (received_at);

CREATE INDEX incoming_events_project_event_idx ON incoming_events(project_id, external_event_id);
CREATE INDEX incoming_events_project_received_idx ON incoming_events(project_id, received_at);

CREATE TABLE conversion_events (
    id                 bigint GENERATED ALWAYS AS IDENTITY,
    project_id         uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    external_event_id  text NOT NULL,
    external_payment_id text NOT NULL,
    external_user_id   text NOT NULL,
    attribution_id     bigint NOT NULL REFERENCES external_user_attributions(id) ON DELETE RESTRICT,
    amount_kopecks     bigint NOT NULL,
    currency           text NOT NULL DEFAULT 'RUB',
    occurred_at        timestamptz NOT NULL,
    processing_note    text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX conversion_events_project_event_idx ON conversion_events(project_id, external_event_id);
CREATE INDEX conversion_events_project_payment_idx ON conversion_events(project_id, external_payment_id);
CREATE INDEX conversion_events_attribution_idx ON conversion_events(attribution_id);
CREATE INDEX conversion_events_occurred_idx ON conversion_events(occurred_at);

CREATE TABLE daily_partner_offer_stats (
    partner_id       uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    offer_id         uuid NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    day              date NOT NULL,
    clicks           integer NOT NULL DEFAULT 0,
    unique_clicks    integer NOT NULL DEFAULT 0,
    registrations    integer NOT NULL DEFAULT 0,
    first_payments   integer NOT NULL DEFAULT 0,
    income_kopecks   bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (partner_id, offer_id, day)
);
CREATE INDEX daily_stats_offer_day_idx ON daily_partner_offer_stats(offer_id, day);

CREATE TABLE daily_tracking_link_stats (
    tracking_link_id uuid NOT NULL REFERENCES tracking_links(id) ON DELETE CASCADE,
    day              date NOT NULL,
    clicks           integer NOT NULL DEFAULT 0,
    unique_clicks    integer NOT NULL DEFAULT 0,
    registrations    integer NOT NULL DEFAULT 0,
    first_payments   integer NOT NULL DEFAULT 0,
    income_kopecks   bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (tracking_link_id, day)
);
CREATE INDEX daily_link_stats_day_idx ON daily_tracking_link_stats(day);

CREATE TABLE tracking_clicks_2026_08 PARTITION OF tracking_clicks
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE incoming_events_2026_08 PARTITION OF incoming_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE conversion_events_2026_08 PARTITION OF conversion_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
