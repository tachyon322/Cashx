-- +goose Up
-- Phase 1: legacy-расширения + B2B роли для паритета с kazik (G6, G10)
-- B2B is_owner/is_admin на partner_profiles, глобальные группы legacy id

-- source_groups: глобальные kazik группы (affiliate_groups) → partner_scoped source_groups
-- Нужен UNIQUE только для не-null, чтобы on-conflict работал для миграции
ALTER TABLE source_groups ADD COLUMN legacy_kazik_group_id text UNIQUE;
CREATE INDEX source_groups_legacy_idx ON source_groups(legacy_kazik_group_id) WHERE legacy_kazik_group_id IS NOT NULL;

-- partner_profiles: роли владельца/админа партнёрки (порт isOwner/isAdmin из affiliate_partners)
ALTER TABLE partner_profiles
    ADD COLUMN is_owner boolean NOT NULL DEFAULT false,
    ADD COLUMN is_admin boolean NOT NULL DEFAULT false;

-- Для audita: комментарий в 00015 "не переносить" теперь устарел — домены/редиректы мигрируем (G2/G3)
-- Ничего не дропаем, просто расширяем.

-- +goose Down
ALTER TABLE partner_profiles DROP COLUMN IF EXISTS is_admin;
ALTER TABLE partner_profiles DROP COLUMN IF EXISTS is_owner;
DROP INDEX IF EXISTS source_groups_legacy_idx;
ALTER TABLE source_groups DROP COLUMN IF EXISTS legacy_kazik_group_id;
