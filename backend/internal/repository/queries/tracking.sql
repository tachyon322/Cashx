-- name: CreateTrackingClick :one
INSERT INTO tracking_clicks (tracking_link_id, ip, user_agent, referrer)
VALUES ($1, $2::inet, $3, $4) RETURNING id, tracking_link_id, ip, user_agent, referrer, created_at;

-- name: GetClickWithLink :one
SELECT c.id, c.tracking_link_id, c.created_at,
       tl.partner_offer_access_id, tl.code,
       a.partner_id, a.offer_id
FROM tracking_clicks c
JOIN tracking_links tl ON tl.id = c.tracking_link_id
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE c.id = $1;

-- name: CreateAttribution :one
INSERT INTO external_user_attributions (project_id, tracking_click_id, partner_id, offer_id, external_user_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, external_user_id) DO NOTHING
RETURNING id, project_id, tracking_click_id, partner_id, offer_id, external_user_id, first_seen_at;

-- name: GetAttributionByProjectUser :one
SELECT id, project_id, tracking_click_id, partner_id, offer_id, external_user_id, first_seen_at
FROM external_user_attributions WHERE project_id = $1 AND external_user_id = $2;

-- name: EventLock :exec
SELECT pg_advisory_xact_lock(hashtextextended($1, 0));

-- name: InsertIncomingEvent :one
INSERT INTO incoming_events (project_id, external_event_id, type, payload, status, reason)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, project_id, external_event_id, type, status, reason, received_at;

-- name: GetIncomingEvent :one
SELECT id, project_id, external_event_id, type, status, reason, received_at
FROM incoming_events WHERE project_id = $1 AND external_event_id = $2;

-- name: InsertConversion :one
INSERT INTO conversion_events (project_id, external_event_id, external_payment_id, external_user_id, attribution_id, amount_kopecks, currency, occurred_at, processing_note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, project_id, external_event_id, external_payment_id, external_user_id, attribution_id, amount_kopecks, currency, occurred_at, processing_note, created_at;

-- name: GetConversionByPayment :one
SELECT id, project_id, external_event_id, external_payment_id, external_user_id, attribution_id, amount_kopecks, currency, occurred_at, processing_note, created_at
FROM conversion_events WHERE project_id = $1 AND external_payment_id = $2;

-- name: GetConversionByEvent :one
SELECT id, project_id, external_event_id, external_payment_id, external_user_id, attribution_id, amount_kopecks, currency, occurred_at, processing_note, created_at
FROM conversion_events WHERE project_id = $1 AND external_event_id = $2;

-- name: UpdateConversionNote :exec
UPDATE conversion_events SET processing_note = $2 WHERE id = $1;

-- name: ListConversionsByAttribution :many
SELECT id, project_id, external_event_id, external_payment_id, external_user_id, attribution_id, amount_kopecks, currency, occurred_at, processing_note, created_at
FROM conversion_events WHERE attribution_id = $1 ORDER BY occurred_at;

-- Daily stats (worker aggregates and cabinet reads).

-- name: UpsertDailyStats :one
INSERT INTO daily_partner_offer_stats (partner_id, offer_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (partner_id, offer_id, day) DO UPDATE
SET clicks = EXCLUDED.clicks,
    unique_clicks = EXCLUDED.unique_clicks,
    registrations = EXCLUDED.registrations,
    first_payments = EXCLUDED.first_payments,
    income_kopecks = EXCLUDED.income_kopecks
RETURNING partner_id, offer_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks;

-- name: DeleteDailyStatsFrom :exec
DELETE FROM daily_partner_offer_stats WHERE day >= $1;

-- name: GetDailyStats :many
SELECT partner_id, offer_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1 AND day >= $2 AND day <= $3
ORDER BY day;

-- name: GetDailyStatsByOffer :many
SELECT partner_id, offer_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1 AND offer_id = $2 AND day >= $3 AND day <= $4
ORDER BY day;

-- name: SumDailyStats :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1 AND day >= $2 AND day <= $3;

-- name: SumDailyStatsByOffer :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1 AND offer_id = $2 AND day >= $3 AND day <= $4;

-- name: SumDailyStatsAllTime :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1;

-- name: SumDailyStatsOfferAllTime :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_partner_offer_stats
WHERE partner_id = $1 AND offer_id = $2;

-- Worker aggregation sources.

-- name: AggClicksByDay :many
SELECT a.partner_id, a.offer_id, (c.created_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(*)::bigint AS clicks
FROM tracking_clicks c
JOIN tracking_links tl ON tl.id = c.tracking_link_id
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE c.created_at >= $1 AND c.created_at < $2
GROUP BY a.partner_id, a.offer_id, day;

-- name: AggUniqueClicksByDay :many
SELECT a.partner_id, a.offer_id, (c.created_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(DISTINCT c.ip)::bigint AS unique_clicks
FROM tracking_clicks c
JOIN tracking_links tl ON tl.id = c.tracking_link_id
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE c.created_at >= $1 AND c.created_at < $2 AND c.ip IS NOT NULL
GROUP BY a.partner_id, a.offer_id, day;

-- name: AggRegistrationsByDay :many
SELECT partner_id, offer_id, (first_seen_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(*)::bigint AS registrations
FROM external_user_attributions
WHERE first_seen_at >= $1 AND first_seen_at < $2
GROUP BY partner_id, offer_id, day;

-- name: AggFirstPaymentsByDay :many
SELECT a.partner_id, a.offer_id, fp.day, count(*)::bigint AS first_payments
FROM (
    SELECT attribution_id, (min(occurred_at) AT TIME ZONE 'Europe/Moscow')::date AS day
    FROM conversion_events
    WHERE occurred_at >= $1 AND occurred_at < $2
    GROUP BY attribution_id
) fp
JOIN external_user_attributions a ON a.id = fp.attribution_id
GROUP BY a.partner_id, a.offer_id, fp.day;

-- name: AggIncomeByDay :many
SELECT partner_id, offer_id, (created_at AT TIME ZONE 'Europe/Moscow')::date AS day, sum(amount_kopecks)::bigint AS income_kopecks
FROM commission_earnings
WHERE created_at >= $1 AND created_at < $2 AND reversed_at IS NULL
GROUP BY partner_id, offer_id, day;

-- Per-source (tracking link) daily stats.

-- name: UpsertDailyLinkStats :one
INSERT INTO daily_tracking_link_stats (tracking_link_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tracking_link_id, day) DO UPDATE
SET clicks = EXCLUDED.clicks,
    unique_clicks = EXCLUDED.unique_clicks,
    registrations = EXCLUDED.registrations,
    first_payments = EXCLUDED.first_payments,
    income_kopecks = EXCLUDED.income_kopecks
RETURNING tracking_link_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks;

-- name: DeleteDailyLinkStatsFrom :exec
DELETE FROM daily_tracking_link_stats WHERE day >= $1;

-- name: SumDailyLinkStats :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_tracking_link_stats
WHERE tracking_link_id = $1 AND day >= $2 AND day <= $3;

-- name: SumDailyLinkStatsAllTime :one
SELECT COALESCE(sum(clicks), 0)::bigint AS clicks,
       COALESCE(sum(unique_clicks), 0)::bigint AS unique_clicks,
       COALESCE(sum(registrations), 0)::bigint AS registrations,
       COALESCE(sum(first_payments), 0)::bigint AS first_payments,
       COALESCE(sum(income_kopecks), 0)::bigint AS income_kopecks
FROM daily_tracking_link_stats
WHERE tracking_link_id = $1;

-- name: AggClicksByLink :many
SELECT tracking_link_id, (created_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(*)::bigint AS clicks
FROM tracking_clicks
WHERE created_at >= $1 AND created_at < $2
GROUP BY tracking_link_id, day;

-- name: AggUniqueClicksByLink :many
SELECT tracking_link_id, (created_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(DISTINCT ip)::bigint AS unique_clicks
FROM tracking_clicks
WHERE created_at >= $1 AND created_at < $2 AND ip IS NOT NULL
GROUP BY tracking_link_id, day;

-- name: AggRegistrationsByLink :many
SELECT c.tracking_link_id, (a.first_seen_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(*)::bigint AS registrations
FROM external_user_attributions a
JOIN tracking_clicks c ON c.id = a.tracking_click_id
WHERE a.first_seen_at >= $1 AND a.first_seen_at < $2
GROUP BY c.tracking_link_id, day;

-- name: AggFirstPaymentsByLink :many
SELECT c.tracking_link_id, fp.day, count(*)::bigint AS first_payments
FROM (
    SELECT attribution_id, (min(occurred_at) AT TIME ZONE 'Europe/Moscow')::date AS day
    FROM conversion_events
    WHERE occurred_at >= $1 AND occurred_at < $2
    GROUP BY attribution_id
) fp
JOIN external_user_attributions a ON a.id = fp.attribution_id
JOIN tracking_clicks c ON c.id = a.tracking_click_id
GROUP BY c.tracking_link_id, fp.day;

-- name: AggIncomeByLink :many
SELECT tracking_link_id, (created_at AT TIME ZONE 'Europe/Moscow')::date AS day, sum(amount_kopecks)::bigint AS income_kopecks
FROM commission_earnings
WHERE created_at >= $1 AND created_at < $2 AND reversed_at IS NULL AND tracking_link_id IS NOT NULL
GROUP BY tracking_link_id, day;

-- Cabinet offer history pieces.

-- name: HistoryClicksByLink :many
SELECT c.id, c.created_at
FROM tracking_clicks c
WHERE c.tracking_link_id = $1 AND c.created_at >= $2 AND c.created_at <= $3
ORDER BY c.created_at DESC LIMIT $4;

-- name: HistoryAttributionsByLink :many
SELECT a.id, a.external_user_id, a.first_seen_at
FROM external_user_attributions a
JOIN tracking_clicks c ON c.id = a.tracking_click_id
WHERE c.tracking_link_id = $1 AND a.first_seen_at >= $2 AND a.first_seen_at <= $3
ORDER BY a.first_seen_at DESC LIMIT $4;

-- name: HistoryConversionsByLink :many
SELECT ce.id, ce.external_user_id, ce.amount_kopecks, ce.occurred_at
FROM conversion_events ce
JOIN external_user_attributions a ON a.id = ce.attribution_id
JOIN tracking_clicks c ON c.id = a.tracking_click_id
WHERE c.tracking_link_id = $1 AND ce.occurred_at >= $2 AND ce.occurred_at <= $3
ORDER BY ce.occurred_at DESC LIMIT $4;

-- name: HistoryEarningsByPartnerOffer :many
SELECT e.id, e.amount_kopecks, e.reversed_at, e.created_at
FROM commission_earnings e
WHERE e.partner_id = $1 AND e.offer_id = $2
  AND e.created_at >= $3 AND e.created_at <= $4
ORDER BY e.created_at DESC LIMIT $5;
