-- +goose Up
-- RevShare is a single partner-level commission rate applied to every offer
-- the partner joins, instead of each offer's own terms rate. Stored in bps
-- (1% = 100 bps), default 40% matches the platform's headline offer.
ALTER TABLE partner_profiles
    ADD COLUMN revshare_percent_bps integer NOT NULL DEFAULT 4000
    CHECK (revshare_percent_bps >= 0 AND revshare_percent_bps <= 10000);

-- +goose Down
ALTER TABLE partner_profiles
    DROP COLUMN revshare_percent_bps;
