-- +goose Up
-- referral_rewards had an index only on (referrer_partner_id, created_at);
-- the per-invited reward sums (the correlated sum in
-- ListReferralsByReferrerWithRewards) filter by invited_partner_id and had
-- to seq-scan the whole table on every lookup. Partial covering index:
-- reversed rows are excluded and sum(amount_kopecks) is served by an
-- index-only scan without heap access.
CREATE INDEX referral_rewards_invited_idx
    ON referral_rewards(invited_partner_id, amount_kopecks)
    WHERE reversed_at IS NULL;

-- +goose Down
DROP INDEX referral_rewards_invited_idx;
