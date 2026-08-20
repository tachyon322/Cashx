-- +goose Up
-- Auto-creates the monthly partition for a partitioned table when a row is
-- inserted whose month has no partition yet.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auto_create_partition()
RETURNS trigger AS $$
DECLARE
  part_name text := TG_TABLE_NAME || '_' || to_char(NEW.created_at, 'YYYY_MM');
  start_d date := date_trunc('month', NEW.created_at)::date;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
    EXECUTE format('CREATE TABLE %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
      part_name, TG_TABLE_NAME, start_d, start_d + interval '1 month');
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE tracking_clicks (
    id                bigint GENERATED ALWAYS AS IDENTITY,
    tracking_link_id  uuid NOT NULL REFERENCES tracking_links(id) ON DELETE RESTRICT,
    ip                inet,
    user_agent        text,
    referrer          text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TRIGGER tracking_clicks_partition_trg
    BEFORE INSERT ON tracking_clicks
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();

CREATE INDEX tracking_clicks_link_created_idx ON tracking_clicks(tracking_link_id, created_at);
CREATE INDEX tracking_clicks_created_idx ON tracking_clicks(created_at);

CREATE TABLE external_user_attributions (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id         uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tracking_click_id  bigint,
    partner_id         uuid,
    offer_id           uuid,
    external_user_id   text NOT NULL,
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, external_user_id)
);
CREATE INDEX attributions_partner_offer_firstseen_idx
    ON external_user_attributions(partner_id, offer_id, first_seen_at);

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

CREATE TRIGGER incoming_events_partition_trg
    BEFORE INSERT ON incoming_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();

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

-- Idempotency is enforced by the application (advisory xact lock + indexed
-- check + insert in one transaction), because UNIQUE constraints on
-- partitioned tables must include the partition key, which a replay would
-- not reproduce. Indexes below make the lookups fast.
CREATE INDEX conversion_events_project_event_idx ON conversion_events(project_id, external_event_id);
CREATE INDEX conversion_events_project_payment_idx ON conversion_events(project_id, external_payment_id);

CREATE TRIGGER conversion_events_partition_trg
    BEFORE INSERT ON conversion_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();

CREATE INDEX conversion_events_attribution_idx ON conversion_events(attribution_id);
CREATE INDEX conversion_events_occurred_idx ON conversion_events(occurred_at);

CREATE TABLE daily_partner_offer_stats (
    partner_id       uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    offer_id         uuid NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    day              date NOT NULL,
    clicks           integer NOT NULL DEFAULT 0,
    registrations    integer NOT NULL DEFAULT 0,
    first_payments   integer NOT NULL DEFAULT 0,
    income_kopecks   bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (partner_id, offer_id, day)
);
CREATE INDEX daily_stats_offer_day_idx ON daily_partner_offer_stats(offer_id, day);

-- Starter partitions for the current month (auto_create_partition covers the rest).
CREATE TABLE tracking_clicks_2026_08 PARTITION OF tracking_clicks
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE incoming_events_2026_08 PARTITION OF incoming_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE conversion_events_2026_08 PARTITION OF conversion_events
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

-- +goose Down
DROP TABLE IF EXISTS tracking_clicks_2026_08;
DROP TABLE IF EXISTS incoming_events_2026_08;
DROP TABLE IF EXISTS conversion_events_2026_08;
DROP TABLE IF EXISTS daily_partner_offer_stats;
DROP TABLE IF EXISTS conversion_events;
DROP TABLE IF EXISTS incoming_events;
DROP TABLE IF EXISTS external_user_attributions;
DROP TABLE IF EXISTS tracking_clicks;
DROP FUNCTION IF EXISTS auto_create_partition();
