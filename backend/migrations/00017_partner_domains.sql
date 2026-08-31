-- +goose Up
-- Phase 1 G2: партнерские домены (порт affiliate_domains из kazik schema.ts:150)
-- Хранит разрешённые origin для построения tracking URL: buildAffiliateLink(code,domain,defaultDomain)

CREATE TABLE partner_domains (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    url                     text NOT NULL,
    is_active               boolean NOT NULL DEFAULT true,
    comment                 text,
    legacy_kazik_domain_id  text UNIQUE,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT partner_domains_url_unique UNIQUE (url)
);

CREATE INDEX partner_domains_is_active_idx ON partner_domains(is_active);
CREATE INDEX partner_domains_legacy_idx ON partner_domains(legacy_kazik_domain_id) WHERE legacy_kazik_domain_id IS NOT NULL;
CREATE INDEX partner_domains_created_at_idx ON partner_domains(created_at);

-- Нормализация origin на уровне приложения (service: normalizeDomainUrl), в БД храним как есть но lowercased приложением.

GRANT SELECT, INSERT, UPDATE, DELETE ON partner_domains TO cashx_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON partner_domains FROM cashx_app;
DROP TABLE IF EXISTS partner_domains;
