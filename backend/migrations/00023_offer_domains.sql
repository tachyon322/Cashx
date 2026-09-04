-- +goose Up
-- Домены оффера: один основной + запасные зеркала. Партнёр выбирает домен
-- на источнике (ссылка строится как {домен}/r/{code}), администратор ведёт
-- список в Каталоге. Основной домен — дефолт для ссылок без явного выбора.

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
-- Ровно один основной домен на оффер.
CREATE UNIQUE INDEX offer_domains_one_main ON offer_domains(offer_id) WHERE is_main;

GRANT SELECT, INSERT, UPDATE, DELETE ON offer_domains TO cashx_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON offer_domains FROM cashx_app;
DROP TABLE IF EXISTS offer_domains;
