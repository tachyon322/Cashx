-- +goose Up
-- Phase 1 G3: взвешенные пулы редиректов (порт affiliate_redirects + affiliate_redirect_urls)
-- kazik schema.ts:24-50, service.ts:194 weightedPick

CREATE TABLE redirect_pools (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                        text NOT NULL,
    comment                     text,
    legacy_kazik_redirect_id    text UNIQUE,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX redirect_pools_legacy_idx ON redirect_pools(legacy_kazik_redirect_id) WHERE legacy_kazik_redirect_id IS NOT NULL;
CREATE INDEX redirect_pools_created_at_idx ON redirect_pools(created_at);

CREATE TABLE redirect_pool_urls (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    redirect_id     uuid NOT NULL REFERENCES redirect_pools(id) ON DELETE CASCADE,
    url             text NOT NULL,
    weight          integer NOT NULL DEFAULT 1 CHECK (weight >= 1),
    is_active       boolean NOT NULL DEFAULT true,
    sort_order      integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX redirect_pool_urls_redirect_id_idx ON redirect_pool_urls(redirect_id);
CREATE INDEX redirect_pool_urls_redirect_sort_idx ON redirect_pool_urls(redirect_id, sort_order);

-- FK из tracking_links.redirect_id → redirect_pools (колонка создана в 00016 без FK)
ALTER TABLE tracking_links
    ADD CONSTRAINT tracking_links_redirect_id_fkey FOREIGN KEY (redirect_id) REFERENCES redirect_pools(id) ON DELETE SET NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON redirect_pools, redirect_pool_urls TO cashx_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON redirect_pools, redirect_pool_urls FROM cashx_app;
ALTER TABLE tracking_links DROP CONSTRAINT IF EXISTS tracking_links_redirect_id_fkey;
DROP TABLE IF EXISTS redirect_pool_urls;
DROP TABLE IF EXISTS redirect_pools;
