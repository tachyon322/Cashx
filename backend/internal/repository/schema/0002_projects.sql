-- Pure DDL mirror of migrations/00003_projects.sql for sqlc.
CREATE TABLE projects (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text NOT NULL UNIQUE,
    name            text NOT NULL,
    description     text,
    logo_media_id   uuid REFERENCES media_assets(id) ON DELETE SET NULL,
    destination_url text NOT NULL,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE integration_keys (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key_id            text NOT NULL UNIQUE,
    secret_ciphertext bytea NOT NULL,
    secret_hint       text NOT NULL,
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    rotated_at        timestamptz,
    last_used_at      timestamptz
);
CREATE INDEX integration_keys_project_id_idx ON integration_keys(project_id);

CREATE TABLE project_settings (
    project_id         uuid PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    allow_new_partners boolean NOT NULL DEFAULT true,
    comment            text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE offers (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            text NOT NULL,
    category        text,
    description     text,
    logo_media_id   uuid REFERENCES media_assets(id) ON DELETE SET NULL,
    destination_url text,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('active', 'available', 'pending', 'coming_soon')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX offers_project_id_idx ON offers(project_id);
CREATE UNIQUE INDEX offers_one_active_per_project ON offers(project_id) WHERE status = 'active';

CREATE TABLE offer_terms_versions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id       uuid NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    version        integer NOT NULL,
    rate_bps       integer NOT NULL,
    effective_from timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (offer_id, version)
);

CREATE TABLE partner_offer_accesses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    offer_id   uuid NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    rate_bps   integer NOT NULL,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (partner_id, offer_id)
);

CREATE TABLE source_groups (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id              uuid NOT NULL REFERENCES partner_profiles(id) ON DELETE CASCADE,
    name                    text NOT NULL,
    comment                 text,
    legacy_kazik_group_id   text,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (partner_id, name)
);
CREATE INDEX source_groups_legacy_kazik_group_id_idx ON source_groups(legacy_kazik_group_id) WHERE legacy_kazik_group_id IS NOT NULL;

CREATE TABLE partner_domains (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    url                     text NOT NULL UNIQUE,
    is_active               boolean NOT NULL DEFAULT true,
    comment                 text,
    legacy_kazik_domain_id  text UNIQUE,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE redirect_pools (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                        text NOT NULL,
    comment                     text,
    legacy_kazik_redirect_id    text UNIQUE,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE redirect_pool_urls (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    redirect_id     uuid NOT NULL REFERENCES redirect_pools(id) ON DELETE CASCADE,
    url             text NOT NULL,
    weight          integer NOT NULL DEFAULT 1 CHECK (weight >= 1),
    is_active       boolean NOT NULL DEFAULT true,
    sort_order      integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tracking_links (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_offer_access_id uuid NOT NULL REFERENCES partner_offer_accesses(id) ON DELETE CASCADE,
    code                    text NOT NULL UNIQUE,
    name                    text NOT NULL DEFAULT '',
    comment                 text,
    group_id                uuid REFERENCES source_groups(id) ON DELETE SET NULL,
    is_default              boolean NOT NULL DEFAULT false,
    is_active               boolean NOT NULL DEFAULT true,
    type                    text NOT NULL DEFAULT 'link' CHECK (type IN ('link', 'promo')),
    registration_bonus      integer CHECK (registration_bonus IS NULL OR registration_bonus >= 0),
    domain                  text,
    redirect_id             uuid REFERENCES redirect_pools(id) ON DELETE SET NULL,
    legacy_kazik_source_id  text,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tracking_links_promo_domain_check CHECK (type != 'promo' OR domain IS NULL)
);
CREATE INDEX tracking_links_access_idx ON tracking_links(partner_offer_access_id);
CREATE INDEX tracking_links_group_idx ON tracking_links(group_id);
CREATE UNIQUE INDEX tracking_links_one_default_per_access ON tracking_links(partner_offer_access_id) WHERE is_default;
CREATE INDEX tracking_links_type_idx ON tracking_links(type);
CREATE INDEX tracking_links_domain_idx ON tracking_links(domain) WHERE domain IS NOT NULL;
CREATE INDEX tracking_links_redirect_idx ON tracking_links(redirect_id) WHERE redirect_id IS NOT NULL;
CREATE INDEX tracking_links_legacy_kazik_source_id_idx ON tracking_links(legacy_kazik_source_id) WHERE legacy_kazik_source_id IS NOT NULL;

CREATE TABLE offer_domains (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id   uuid NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    url        text NOT NULL,
    is_main    boolean NOT NULL DEFAULT false,
    is_active  boolean NOT NULL DEFAULT true,
    comment    text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT offer_domains_offer_url_unique UNIQUE (offer_id, url)
);
CREATE INDEX offer_domains_offer_idx ON offer_domains(offer_id);
CREATE UNIQUE INDEX offer_domains_one_main ON offer_domains(offer_id) WHERE is_main;
