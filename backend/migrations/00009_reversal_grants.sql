-- +goose Up
-- The revenue reversal pipeline marks commission_earnings/referral_rewards as
-- reversed (UPDATE), which the 00006 grants (SELECT, INSERT only on financial
-- rows) did not cover; reversals 500'd as the app role.
GRANT UPDATE ON commission_earnings, referral_rewards TO cashx_app;

-- +goose Down
REVOKE UPDATE ON commission_earnings, referral_rewards FROM cashx_app;
