-- +goose Up
-- Root cause of the 2026-09-01 ingestion outage, established on PG 17:
--   * A BEFORE ROW trigger on a partitioned table fires AFTER the row is
--     routed to a partition. When no partition exists for the row's month,
--     routing fails with SQLSTATE 23514 before the trigger ever runs — so
--     auto_create_partition() could never create a missing partition.
--   * A BEFORE STATEMENT trigger cannot do it either: attaching a partition
--     takes a lock the in-flight INSERT already holds in the same session
--     (SQLSTATE 55006).
-- Net: no trigger can create partitions for the table being inserted into.
--
-- Correct approach: create partitions AHEAD of time + self-heal on 23514.
--   * ensure_partitions_range(back, ahead) — SECURITY DEFINER, creates the
--     monthly partitions of all three tables for [now-back, now+ahead].
--     Called by the worker every 6h with (36, 2) and by cmd/migrate-kazik
--     before inserting historical rows.
--   * ensure_partitions_for(ts) — ensures the month of ts plus the current
--     and next month. Called from Go right before event inserts and as the
--     23514-retry remedy for click inserts.
--   * The old (ineffective) row triggers are dropped.
--
-- Inserts routed through the parent only need INSERT on the parent (Postgres
-- does not check per-partition privileges for routed inserts), so no extra
-- GRANTs for cashx_app are required.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_partitions_range(months_back int, months_ahead int)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  t record;
  m int;
  start_d date;
  part_name text;
BEGIN
  IF months_back < 0 OR months_back > 120 OR months_ahead < 0 OR months_ahead > 24 THEN
    RAISE EXCEPTION 'partition range out of bounds: back=% ahead=%', months_back, months_ahead;
  END IF;
  FOR t IN
    SELECT * FROM (VALUES
      ('tracking_clicks',  'created_at'),
      ('incoming_events',  'received_at'),
      ('conversion_events','created_at')
    ) AS v(parent_name, ts_col)
  LOOP
    FOR m IN -months_back..months_ahead LOOP
      start_d := date_trunc('month', now() + make_interval(months => m))::date;
      part_name := t.parent_name || '_' || to_char(start_d, 'YYYY_MM');
      IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
        -- Explicitly qualified: CREATE TABLE without a schema targets the
        -- first schema of search_path, which is pg_catalog here and is not
        -- writable.
        EXECUTE format('CREATE TABLE public.%I PARTITION OF public.%I FOR VALUES FROM (%L) TO (%L)',
          part_name, t.parent_name, start_d, start_d + interval '1 month');
      END IF;
    END LOOP;
  END LOOP;
END $$;

CREATE OR REPLACE FUNCTION ensure_partitions_for(ts timestamptz)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  t record;
  start_d date;
  part_name text;
BEGIN
  FOR t IN
    SELECT * FROM (VALUES
      ('tracking_clicks',  'created_at'),
      ('incoming_events',  'received_at'),
      ('conversion_events','created_at')
    ) AS v(parent_name, ts_col)
  LOOP
    -- The month of ts (e.g. a backdated revenue event) ...
    start_d := date_trunc('month', ts)::date;
    part_name := t.parent_name || '_' || to_char(start_d, 'YYYY_MM');
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
      EXECUTE format('CREATE TABLE public.%I PARTITION OF public.%I FOR VALUES FROM (%L) TO (%L)',
        part_name, t.parent_name, start_d, start_d + interval '1 month');
    END IF;
    -- ... plus the current and next month (received_at/created_at default
    -- to now()).
    start_d := date_trunc('month', now())::date;
    part_name := t.parent_name || '_' || to_char(start_d, 'YYYY_MM');
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
      EXECUTE format('CREATE TABLE public.%I PARTITION OF public.%I FOR VALUES FROM (%L) TO (%L)',
        part_name, t.parent_name, start_d, start_d + interval '1 month');
    END IF;
    start_d := (date_trunc('month', now()) + interval '1 month')::date;
    part_name := t.parent_name || '_' || to_char(start_d, 'YYYY_MM');
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
      EXECUTE format('CREATE TABLE public.%I PARTITION OF public.%I FOR VALUES FROM (%L) TO (%L)',
        part_name, t.parent_name, start_d, start_d + interval '1 month');
    END IF;
  END LOOP;
END $$;
-- +goose StatementEnd

-- The row triggers never created missing partitions; drop them so inserts
-- stop paying dead trigger overhead.
DROP TRIGGER tracking_clicks_partition_trg ON tracking_clicks;
DROP TRIGGER incoming_events_partition_trg ON incoming_events;
DROP TRIGGER conversion_events_partition_trg ON conversion_events;
DROP FUNCTION IF EXISTS auto_create_partition();

-- Backfill the current window (36 months back, 2 ahead) immediately — this
-- also covers hosts where the 2026-09 partitions had to be made by hand.
SELECT ensure_partitions_range(36, 2);

-- +goose Down
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
    RETURN NEW;
  END IF;
  EXECUTE format('SELECT ($1).%I', ts_col) INTO ts USING NEW;
  part_name := parent_name || '_' || to_char(ts, 'YYYY_MM');
  start_d := date_trunc('month', ts)::date;
  IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = part_name) THEN
    EXECUTE format('CREATE TABLE public.%I PARTITION OF public.%I FOR VALUES FROM (%L) TO (%L)',
      part_name, parent_name, start_d, start_d + interval '1 month');
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tracking_clicks_partition_trg
    BEFORE INSERT ON tracking_clicks
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('tracking_clicks', 'created_at');
CREATE TRIGGER incoming_events_partition_trg
    BEFORE INSERT ON incoming_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('incoming_events', 'received_at');
CREATE TRIGGER conversion_events_partition_trg
    BEFORE INSERT ON conversion_events
    FOR EACH ROW EXECUTE FUNCTION auto_create_partition('conversion_events', 'created_at');

DROP FUNCTION IF EXISTS ensure_partitions_for(timestamptz);
DROP FUNCTION IF EXISTS ensure_partitions_range(int, int);
