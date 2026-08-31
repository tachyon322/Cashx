-- +goose Up
-- Phase 1: promo/link type, registration bonus, domain, redirect_id
-- kazik affiliate_sources.type='link'|'promo', registration_bonus, domain, redirect_id
-- + tracking_links расширяется для паритета с kazik (G1-G3)

ALTER TABLE tracking_links
    ADD COLUMN type text NOT NULL DEFAULT 'link' CHECK (type IN ('link', 'promo')),
    ADD COLUMN registration_bonus integer CHECK (registration_bonus IS NULL OR registration_bonus >= 0),
    ADD COLUMN domain text,
    ADD COLUMN redirect_id uuid;

-- Promo не использует домен (kazik service.ts:resolveSourceDomain returns null for type=promo)
ALTER TABLE tracking_links
    ADD CONSTRAINT tracking_links_promo_domain_check CHECK (type != 'promo' OR domain IS NULL);

-- Индексы для выборок как в kazik listSources (type, domain, redirect)
CREATE INDEX tracking_links_type_idx ON tracking_links(type);
CREATE INDEX tracking_links_domain_idx ON tracking_links(domain) WHERE domain IS NOT NULL;
CREATE INDEX tracking_links_redirect_idx ON tracking_links(redirect_id) WHERE redirect_id IS NOT NULL;

-- Существующие ссылки — все 'link' (default покрывает), ничего не менять.

-- +goose Down
DROP INDEX IF EXISTS tracking_links_redirect_idx;
DROP INDEX IF EXISTS tracking_links_domain_idx;
DROP INDEX IF EXISTS tracking_links_type_idx;
ALTER TABLE tracking_links DROP CONSTRAINT IF EXISTS tracking_links_promo_domain_check;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS redirect_id;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS domain;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS registration_bonus;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS type;
