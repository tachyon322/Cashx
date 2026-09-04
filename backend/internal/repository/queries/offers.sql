-- name: CreateOffer :one
INSERT INTO offers (project_id, name, category, description, destination_url, status)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, project_id, name, category, description, destination_url, status, created_at, updated_at;

-- name: GetOfferByID :one
SELECT id, project_id, name, category, description, destination_url, status, created_at, updated_at FROM offers WHERE id = $1;

-- name: UpdateOffer :one
UPDATE offers
SET name = COALESCE(sqlc.narg('name'), name),
    category = COALESCE(sqlc.narg('category'), category),
    description = COALESCE(sqlc.narg('description'), description),
    destination_url = COALESCE(sqlc.narg('destination_url'), destination_url),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, project_id, name, category, description, destination_url, status, created_at, updated_at;

-- name: ListOffers :many
SELECT o.id, o.project_id, o.name, o.category, o.description, o.destination_url, o.status, o.created_at, o.updated_at,
       p.name AS project_name
FROM offers o JOIN projects p ON p.id = o.project_id
WHERE ($1 = '' OR o.project_id = $1::uuid)
ORDER BY o.created_at DESC LIMIT $2 OFFSET $3;

-- name: CountOffers :one
SELECT count(*) FROM offers WHERE ($1 = '' OR project_id = $1::uuid);

-- name: ListOffersByProject :many
SELECT id, project_id, name, category, description, destination_url, status, created_at, updated_at
FROM offers WHERE project_id = $1 ORDER BY created_at DESC;

-- name: GetOfferWithProject :one
SELECT o.id, o.project_id, o.name, o.category, o.description, o.destination_url, o.status, o.created_at, o.updated_at,
       p.name AS project_name, p.logo_media_id, p.destination_url AS project_destination_url
FROM offers o JOIN projects p ON p.id = o.project_id
WHERE o.id = $1;

-- name: GetCurrentTerms :one
SELECT id, offer_id, version, rate_bps, effective_from, created_at
FROM offer_terms_versions WHERE offer_id = $1 ORDER BY version DESC LIMIT 1;

-- name: CreateOfferTerms :one
INSERT INTO offer_terms_versions (offer_id, version, rate_bps, effective_from)
VALUES ($1, $2, $3, $4) RETURNING id, offer_id, version, rate_bps, effective_from, created_at;

-- name: CountOfferTerms :one
SELECT count(*) FROM offer_terms_versions WHERE offer_id = $1;

-- name: CreatePartnerAccess :one
INSERT INTO partner_offer_accesses (partner_id, offer_id, rate_bps, status)
VALUES ($1, $2, $3, 'active') RETURNING id, partner_id, offer_id, rate_bps, status, created_at, updated_at;

-- name: GetPartnerAccess :one
SELECT id, partner_id, offer_id, rate_bps, status, created_at, updated_at
FROM partner_offer_accesses WHERE partner_id = $1 AND offer_id = $2;

-- name: UpdatePartnerAccessRate :one
UPDATE partner_offer_accesses SET rate_bps = $3, updated_at = now()
WHERE partner_id = $1 AND offer_id = $2
RETURNING id, partner_id, offer_id, rate_bps, status, created_at, updated_at;

-- name: ListPartnerAccesses :many
SELECT id, partner_id, offer_id, rate_bps, status, created_at, updated_at
FROM partner_offer_accesses WHERE partner_id = $1 ORDER BY created_at;

-- name: ListPartnerAccessesWithOffer :many
SELECT a.id, a.partner_id, a.offer_id, a.rate_bps, a.status, a.created_at, a.updated_at,
       o.name AS offer_name, o.project_id, o.status AS offer_status
FROM partner_offer_accesses a JOIN offers o ON o.id = a.offer_id
WHERE a.partner_id = $1 ORDER BY a.created_at;

-- name: ListAllActiveAccesses :many
SELECT a.id, a.partner_id, a.offer_id, a.rate_bps, a.status
FROM partner_offer_accesses a WHERE a.status = 'active';

-- name: CreateTrackingLink :one
INSERT INTO tracking_links (partner_offer_access_id, code, name, comment, group_id, is_default)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTrackingLinkByCode :one
SELECT tl.id, tl.partner_offer_access_id, tl.code, tl.is_active, tl.type, tl.domain, tl.redirect_id, tl.registration_bonus, tl.created_at,
       a.partner_id, a.offer_id, a.status AS access_status,
       o.destination_url AS offer_destination_url,
       p.destination_url AS project_destination_url,
       (tl.domain IS NOT NULL AND EXISTS (
           SELECT 1 FROM offer_domains od
           WHERE od.offer_id = a.offer_id AND od.url = tl.domain AND od.is_active
       )) AS domain_active
FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
JOIN offers o ON o.id = a.offer_id
JOIN projects p ON p.id = o.project_id
WHERE tl.code = $1;

-- name: GetDefaultTrackingLinkByAccessID :one
SELECT * FROM tracking_links
WHERE partner_offer_access_id = $1
ORDER BY is_default DESC, created_at
LIMIT 1;

-- name: ListDefaultTrackingLinksByPartner :many
-- Default link per access for all of the partner's accesses (same
-- is_default DESC, created_at preference as GetDefaultTrackingLinkByAccessID)
-- in a single query, instead of one query per offer in Summary/ListOffers.
SELECT DISTINCT ON (tl.partner_offer_access_id) tl.*
FROM tracking_links tl
JOIN partner_offer_accesses pa ON pa.id = tl.partner_offer_access_id
WHERE pa.partner_id = $1
ORDER BY tl.partner_offer_access_id, tl.is_default DESC, tl.created_at;

-- name: GetTrackingLinkByID :one
SELECT * FROM tracking_links WHERE id = $1;

-- name: ListTrackingLinksByAccessID :many
SELECT tl.*, g.name AS group_name
FROM tracking_links tl
LEFT JOIN source_groups g ON g.id = tl.group_id
WHERE tl.partner_offer_access_id = $1
ORDER BY tl.is_default DESC, tl.created_at;

-- name: ListTrackingLinksByPartnerWithOffer :many
-- All tracking links of a partner across every offer they have access to,
-- with the offer id and group name. Backs GET /cabinet/sources (dashboard
-- needs all sources in one round trip instead of one request per offer).
SELECT tl.*, g.name AS group_name, pa.offer_id
FROM tracking_links tl
JOIN partner_offer_accesses pa ON pa.id = tl.partner_offer_access_id
LEFT JOIN source_groups g ON g.id = tl.group_id
WHERE pa.partner_id = $1
ORDER BY tl.created_at;

-- name: UpdateTrackingLink :one
UPDATE tracking_links
SET name = $2,
    code = $3,
    comment = $4,
    group_id = $5,
    is_active = $6,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearDefaultTrackingLinks :exec
UPDATE tracking_links SET is_default = false, updated_at = now()
WHERE partner_offer_access_id = $1;

-- name: SetDefaultTrackingLink :exec
UPDATE tracking_links SET is_default = true, updated_at = now()
WHERE id = $1;

-- name: DeleteTrackingLink :exec
DELETE FROM tracking_links WHERE id = $1;

-- name: CountActiveLinksByAccess :one
SELECT count(*) FROM tracking_links WHERE partner_offer_access_id = $1 AND is_active = true;

-- name: CountClicksByLink :one
SELECT count(*) FROM tracking_clicks WHERE tracking_link_id = $1;

-- name: CreateSourceGroup :one
INSERT INTO source_groups (partner_id, name, comment)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListSourceGroupsByPartner :many
SELECT * FROM source_groups WHERE partner_id = $1 ORDER BY name;

-- name: GetSourceGroupByID :one
SELECT * FROM source_groups WHERE id = $1;

-- name: UpdateSourceGroup :one
UPDATE source_groups SET name = $2, comment = $3, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteSourceGroup :exec
DELETE FROM source_groups WHERE id = $1;

-- name: CountSourcesInGroup :one
SELECT count(*) FROM tracking_links WHERE group_id = $1;

-- name: UpdateAllAccessRatesByPartner :exec
UPDATE partner_offer_accesses SET rate_bps = $2, updated_at = now() WHERE partner_id = $1;

-- name: GetAccessByID :one
SELECT id, partner_id, offer_id, rate_bps, status, created_at, updated_at FROM partner_offer_accesses WHERE id = $1;

-- name: CreateTrackingLinkFull :one
INSERT INTO tracking_links (partner_offer_access_id, code, name, comment, group_id, is_default, is_active, type, registration_bonus, domain, redirect_id, legacy_kazik_source_id)
VALUES (sqlc.arg('partner_offer_access_id'), sqlc.arg('code'), sqlc.arg('name'), sqlc.narg('comment'), sqlc.narg('group_id'), sqlc.arg('is_default'), sqlc.arg('is_active'), COALESCE(sqlc.narg('type')::text, 'link'), sqlc.narg('registration_bonus'), sqlc.narg('domain'), sqlc.narg('redirect_id'), sqlc.narg('legacy_kazik_source_id'))
RETURNING *;

-- name: UpdateTrackingLinkFull :one
UPDATE tracking_links
SET name = COALESCE(sqlc.narg('name'), name),
    code = COALESCE(sqlc.narg('code'), code),
    comment = COALESCE(sqlc.narg('comment'), comment),
    group_id = COALESCE(sqlc.narg('group_id'), group_id),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    type = COALESCE(sqlc.narg('type'), type),
    registration_bonus = COALESCE(sqlc.narg('registration_bonus'), registration_bonus),
    domain = COALESCE(sqlc.narg('domain'), domain),
    redirect_id = COALESCE(sqlc.narg('redirect_id'), redirect_id),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: GetTrackingLinkByCodeExtended :one
SELECT * FROM tracking_links WHERE code = $1;

-- name: GetTrackingLinkByCodeForProject :one
-- Resolve a source (tracking link) by its code within one project's scope.
-- Used by the integrations API: promo-code attribution (registration.created
-- with source_code instead of click_token) and the source lookup endpoint.
SELECT tl.id, tl.code, tl.name, tl.type, tl.is_active, tl.registration_bonus,
       a.partner_id, a.offer_id, a.status AS access_status
FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
JOIN offers o ON o.id = a.offer_id
WHERE tl.code = $1 AND o.project_id = $2
ORDER BY tl.is_active DESC, tl.created_at
LIMIT 1;

-- name: ResolvePromoCode :one
SELECT * FROM tracking_links WHERE code = $1 AND type = 'promo' AND is_active = true LIMIT 1;

-- name: GetRegistrationBonusByCode :one
SELECT registration_bonus FROM tracking_links WHERE code = $1 AND is_active = true LIMIT 1;

-- name: ListTrackingLinksByPartner :many
SELECT tl.* FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE a.partner_id = $1
ORDER BY tl.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountTrackingLinksByPartner :one
SELECT count(*) FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE a.partner_id = $1;

-- name: SearchTrackingLinks :many
SELECT tl.* FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
WHERE a.partner_id = $1
  AND ($2 = '' OR tl.name ILIKE '%' || $2 || '%' OR tl.code ILIKE '%' || $2 || '%')
  AND ($3 = '' OR tl.type = $3)
  AND ($4::uuid IS NULL OR tl.group_id = $4)
  AND ($5::uuid IS NULL OR tl.redirect_id = $5)
ORDER BY tl.created_at DESC
LIMIT $6 OFFSET $7;
