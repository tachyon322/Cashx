-- +goose Up
-- Unique clicks: one IP per day per offer, aggregated by the worker into
-- daily_partner_offer_stats. The raw tracking_clicks.ip is already stored.
ALTER TABLE daily_partner_offer_stats ADD COLUMN unique_clicks integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE daily_partner_offer_stats DROP COLUMN IF EXISTS unique_clicks;
