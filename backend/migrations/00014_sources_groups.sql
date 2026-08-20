-- +goose Up
-- Источники трафика: несколько именованных ссылок на один доступ партнёра
-- к офферу, группировка по потокам и статистика по каждому источнику.

CREATE TABLE source_groups (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    name       text NOT NULL,
    comment    text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (partner_id, name)
);

-- tracking_links: строки таблицы становятся «источниками». Убираем жёсткую
-- связь «одна ссылка на доступ», добавляем имя/заметку/поток и флаг
-- дефолтного источника.
ALTER TABLE tracking_links DROP CONSTRAINT IF EXISTS tracking_links_partner_offer_access_id_key;

ALTER TABLE tracking_links
    ADD COLUMN name       text NOT NULL DEFAULT '',
    ADD COLUMN comment    text,
    ADD COLUMN group_id   uuid REFERENCES source_groups(id) ON DELETE SET NULL,
    ADD COLUMN is_default boolean NOT NULL DEFAULT false;

CREATE INDEX tracking_links_access_idx ON tracking_links(partner_offer_access_id);
CREATE INDEX tracking_links_group_idx ON tracking_links(group_id);
-- Не более одного дефолтного источника на доступ.
CREATE UNIQUE INDEX tracking_links_one_default_per_access
    ON tracking_links(partner_offer_access_id) WHERE is_default;

-- Существующие ссылки (по одной на доступ) становятся дефолтными источниками.
UPDATE tracking_links SET name = 'Основной источник', is_default = true;

-- Материализованная статистика по источнику (пересчитывается воркером).
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

-- Денормализация источника в начислении: доход по источнику считается
-- простым group-by, а не join'ом через конверсию → атрибуцию → клик.
ALTER TABLE commission_earnings
    ADD COLUMN tracking_link_id uuid REFERENCES tracking_links(id) ON DELETE SET NULL;
CREATE INDEX commission_earnings_link_created_idx ON commission_earnings(tracking_link_id, created_at);

-- Права прикладной роли на новые таблицы и на удаление источников.
GRANT SELECT, INSERT, UPDATE, DELETE ON source_groups, daily_tracking_link_stats TO cashx_app;
GRANT DELETE ON tracking_links TO cashx_app;

-- +goose Down
REVOKE DELETE ON tracking_links FROM cashx_app;
REVOKE SELECT, INSERT, UPDATE, DELETE ON source_groups, daily_tracking_link_stats FROM cashx_app;
DROP INDEX IF EXISTS commission_earnings_link_created_idx;
ALTER TABLE commission_earnings DROP COLUMN IF EXISTS tracking_link_id;
DROP TABLE IF EXISTS daily_tracking_link_stats;
ALTER TABLE tracking_links DROP CONSTRAINT IF EXISTS tracking_links_group_id_fkey;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS is_default;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS group_id;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS comment;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS name;
DROP INDEX IF EXISTS tracking_links_one_default_per_access;
DROP INDEX IF EXISTS tracking_links_group_idx;
DROP INDEX IF EXISTS tracking_links_access_idx;
ALTER TABLE tracking_links ADD CONSTRAINT tracking_links_partner_offer_access_id_key UNIQUE (partner_offer_access_id);
DROP TABLE IF EXISTS source_groups;
