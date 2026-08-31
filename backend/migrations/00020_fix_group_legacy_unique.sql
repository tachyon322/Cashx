-- +goose Up
-- Fix 00019: legacy_kazik_group_id был UNIQUE глобально, но source_groups per-partner.
-- Второй партнёр с той же kazik группой падал duplicate. Снимаем UNIQUE, оставляем partial index.

ALTER TABLE source_groups DROP CONSTRAINT IF EXISTS source_groups_legacy_kazik_group_id_key;
DROP INDEX IF EXISTS source_groups_legacy_kazik_group_id_key;
DROP INDEX IF EXISTS source_groups_legacy_idx;
CREATE INDEX source_groups_legacy_kazik_group_id_idx ON source_groups(legacy_kazik_group_id) WHERE legacy_kazik_group_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS source_groups_legacy_kazik_group_id_idx;
CREATE UNIQUE INDEX source_groups_legacy_kazik_group_id_key ON source_groups(legacy_kazik_group_id) WHERE legacy_kazik_group_id IS NOT NULL;
