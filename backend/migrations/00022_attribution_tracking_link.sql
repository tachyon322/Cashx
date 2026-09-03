-- +goose Up
-- Promo (type=promo) attributions have no click: users register with a promo
-- code (?ref=PROMO), so there is no tracking_clicks row. Until now
-- external_user_attributions could only be tied to a source through its
-- click, which made promo registrations invisible in per-source stats
-- (AggRegistrationsByLink and friends join through tracking_clicks).
-- Add tracking_link_id to the attribution itself and backfill it from the
-- first-touch click.
ALTER TABLE external_user_attributions ADD COLUMN tracking_link_id uuid REFERENCES tracking_links(id) ON DELETE SET NULL;

UPDATE external_user_attributions a
SET tracking_link_id = c.tracking_link_id
FROM tracking_clicks c
WHERE c.id = a.tracking_click_id
  AND a.tracking_link_id IS NULL;

CREATE INDEX attributions_link_firstseen_idx
    ON external_user_attributions(tracking_link_id, first_seen_at);

-- The worker (app role) rewrites daily_partner_offer_stats with a
-- delete+reinsert pattern (tracking.RecomputeDailyStats) but only had
-- SELECT/INSERT/UPDATE from 00006 — the DELETE always failed under the app
-- role. Required both by the live 3-day loop and cmd/backfill-stats.
GRANT DELETE ON daily_partner_offer_stats TO cashx_app;

-- +goose Down
REVOKE DELETE ON daily_partner_offer_stats FROM cashx_app;
DROP INDEX IF EXISTS attributions_link_firstseen_idx;
ALTER TABLE external_user_attributions DROP COLUMN IF EXISTS tracking_link_id;
