-- +goose Up
-- Kazik legacy traceability: nullable, не ломает reusable.
ALTER TABLE partner_profiles ADD COLUMN legacy_kazik_partner_id text UNIQUE;
ALTER TABLE wallets ADD COLUMN legacy_kazik_balance integer;
ALTER TABLE tracking_links ADD COLUMN legacy_kazik_source_id text;

-- Индекс для быстрого поиска по legacy source id (для верификации и идемпотентного ре-рана)
CREATE INDEX tracking_links_legacy_kazik_source_id_idx ON tracking_links(legacy_kazik_source_id) WHERE legacy_kazik_source_id IS NOT NULL;

-- Withdrawals legacy id для идемпотентного mirror (используется cashx sync)
ALTER TABLE withdrawal_requests ADD COLUMN legacy_kazik_withdrawal_id text UNIQUE;
CREATE INDEX withdrawal_requests_legacy_kazik_withdrawal_id_idx ON withdrawal_requests(legacy_kazik_withdrawal_id) WHERE legacy_kazik_withdrawal_id IS NOT NULL;

-- Для отладки миграции, NULL-able, не ломает reusable.
-- affiliateDomains/redirects — не переносить, оставить в kazik как deprecated.

-- +goose Down
DROP INDEX IF EXISTS withdrawal_requests_legacy_kazik_withdrawal_id_idx;
ALTER TABLE withdrawal_requests DROP COLUMN IF EXISTS legacy_kazik_withdrawal_id;
DROP INDEX IF EXISTS tracking_links_legacy_kazik_source_id_idx;
ALTER TABLE tracking_links DROP COLUMN IF EXISTS legacy_kazik_source_id;
ALTER TABLE wallets DROP COLUMN IF EXISTS legacy_kazik_balance;
ALTER TABLE partner_profiles DROP COLUMN IF EXISTS legacy_kazik_partner_id;
