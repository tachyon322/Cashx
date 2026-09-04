-- +goose Up
-- Индекс под B2C-рефералов (GET /cabinet/b2c-referrals): дедупликация
-- атрибуций по external_user_id внутри партнёра (DISTINCT ON + сортировка
-- по first_seen_at). Раньше тот же lookup делался отдельным запросом на
-- каждого игрока без индекса — страница грузилась минутами.

CREATE INDEX attributions_partner_user_firstseen_idx
    ON external_user_attributions(partner_id, external_user_id, first_seen_at);

-- +goose Down
DROP INDEX IF EXISTS attributions_partner_user_firstseen_idx;
