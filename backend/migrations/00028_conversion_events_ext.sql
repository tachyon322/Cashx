-- +goose Up
ALTER TABLE conversion_events ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'deposit';
ALTER TABLE conversion_events ADD COLUMN IF NOT EXISTS reversed_at timestamptz;
GRANT UPDATE ON conversion_events TO cashx_app;

-- +goose Down
ALTER TABLE conversion_events DROP COLUMN IF EXISTS reversed_at;
ALTER TABLE conversion_events DROP COLUMN IF EXISTS kind;
