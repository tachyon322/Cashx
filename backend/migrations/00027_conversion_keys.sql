-- +goose Up
CREATE TABLE IF NOT EXISTS conversion_event_payments (
    project_id           uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    external_payment_id  text NOT NULL,
    conversion_event_id  bigint,
    created_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, external_payment_id)
);
GRANT SELECT, INSERT, UPDATE ON conversion_event_payments TO cashx_app;

CREATE TABLE IF NOT EXISTS incoming_event_keys (
    project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    external_event_id text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, external_event_id)
);
GRANT SELECT, INSERT, UPDATE ON incoming_event_keys TO cashx_app;

-- Backfill:
INSERT INTO conversion_event_payments (project_id, external_payment_id, conversion_event_id, created_at)
SELECT project_id, external_payment_id, id, created_at
FROM conversion_events
ON CONFLICT (project_id, external_payment_id) DO NOTHING;

INSERT INTO incoming_event_keys (project_id, external_event_id, created_at)
SELECT project_id, external_event_id, received_at
FROM incoming_events
ON CONFLICT (project_id, external_event_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS incoming_event_keys;
DROP TABLE IF EXISTS conversion_event_payments;
