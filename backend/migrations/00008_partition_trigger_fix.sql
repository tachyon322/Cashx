-- +goose Up
-- Fix auto_create_partition():
-- 1. When a row routes to an existing partition, the BEFORE ROW trigger on the
--    partitioned table fires with TG_TABLE_NAME set to that partition; the old
--    function then tried to create "<partition>_<month>" as a partition OF the
--    partition (permission denied for the app role, junk tables as owner).
--    The function now only acts when the insert targets the parent, and the
--    parent name is passed explicitly via TG_ARGV[0].
-- 2. The timestamp column differs per table (created_at vs received_at for
--    incoming_events); it is passed via TG_ARGV[1].
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auto_create_partition()
RETURNS trigger AS $$
DECLARE
  parent_name text := TG_ARGV[0];
  ts_col      text := TG_ARGV[1];
  ts          timestamptz;
  part_name   text;
  start_d     date;
BEGIN
  IF TG_TABLE_NAME <> parent_name THEN
    -- Row already routed into an existing partition; nothing to create.
    RETURN NEW;
  END IF;
  EXECUTE format('SELECT ($1).%I', ts_col) INTO ts USING NEW;
  part_name := parent_name || '_' || to_char(ts, 'YYYY_MM');
  start_d := date_trunc('month', ts)::date;
  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
    EXECUTE format('CREATE TABLE %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
      part_name, parent_name, start_d, start_d + interval '1 month');
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER tracking_clicks_partition_trg ON tracking_clicks;
CREATE TRIGGER tracking_clicks_partition_trg
    BEFORE INSERT ON tracking_clicks
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('tracking_clicks', 'created_at');

DROP TRIGGER incoming_events_partition_trg ON incoming_events;
CREATE TRIGGER incoming_events_partition_trg
    BEFORE INSERT ON incoming_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('incoming_events', 'received_at');

DROP TRIGGER conversion_events_partition_trg ON conversion_events;
CREATE TRIGGER conversion_events_partition_trg
    BEFORE INSERT ON conversion_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('conversion_events', 'created_at');

-- +goose Down
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

DROP TRIGGER tracking_clicks_partition_trg ON tracking_clicks;
CREATE TRIGGER tracking_clicks_partition_trg
    BEFORE INSERT ON tracking_clicks
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();

DROP TRIGGER incoming_events_partition_trg ON incoming_events;
CREATE TRIGGER incoming_events_partition_trg
    BEFORE INSERT ON incoming_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();

DROP TRIGGER conversion_events_partition_trg ON conversion_events;
CREATE TRIGGER conversion_events_partition_trg
    BEFORE INSERT ON conversion_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition();
