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
RETURNING id, partner_offer_access_id, code, name, comment, group_id, is_default, is_active, created_at, updated_at;

-- name: GetTrackingLinkByCode :one
SELECT tl.id, tl.partner_offer_access_id, tl.code, tl.is_active, tl.created_at,
       a.partner_id, a.offer_id, a.status AS access_status,
       o.destination_url AS offer_destination_url,
       p.destination_url AS project_destination_url
FROM tracking_links tl
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
JOIN offers o ON o.id = a.offer_id
JOIN projects p ON p.id = o.project_id
WHERE tl.code = $1;

-- name: GetDefaultTrackingLinkByAccessID :one
SELECT id, partner_offer_access_id, code, name, comment, group_id, is_default, is_active, created_at, updated_at
FROM tracking_links
WHERE partner_offer_access_id = $1
ORDER BY is_default DESC, created_at
LIMIT 1;

-- name: GetTrackingLinkByID :one
SELECT id, partner_offer_access_id, code, name, comment, group_id, is_default, is_active, created_at, updated_at
FROM tracking_links WHERE id = $1;

-- name: ListTrackingLinksByAccessID :many
SELECT tl.id, tl.partner_offer_access_id, tl.code, tl.name, tl.comment, tl.group_id, tl.is_default, tl.is_active, tl.created_at, tl.updated_at,
       g.name AS group_name
FROM tracking_links tl
LEFT JOIN source_groups g ON g.id = tl.group_id
WHERE tl.partner_offer_access_id = $1
ORDER BY tl.is_default DESC, tl.created_at;

-- name: UpdateTrackingLink :one
UPDATE tracking_links
SET name = $2,
    code = $3,
    comment = $4,
    group_id = $5,
    is_active = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, partner_offer_access_id, code, name, comment, group_id, is_default, is_active, created_at, updated_at;

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
VALUES ($1, $2, $3) RETURNING id, partner_id, name, comment, created_at, updated_at;

-- name: ListSourceGroupsByPartner :many
SELECT id, partner_id, name, comment, created_at, updated_at
FROM source_groups WHERE partner_id = $1 ORDER BY name;

-- name: GetSourceGroupByID :one
SELECT id, partner_id, name, comment, created_at, updated_at
FROM source_groups WHERE id = $1;

-- name: UpdateSourceGroup :one
UPDATE source_groups SET name = $2, comment = $3, updated_at = now()
WHERE id = $1 RETURNING id, partner_id, name, comment, created_at, updated_at;

-- name: DeleteSourceGroup :exec
DELETE FROM source_groups WHERE id = $1;

-- name: CountSourcesInGroup :one
SELECT count(*) FROM tracking_links WHERE group_id = $1;

-- name: GetAccessByID :one
SELECT id, partner_id, offer_id, rate_bps, status, created_at, updated_at FROM partner_offer_accesses WHERE id = $1;
