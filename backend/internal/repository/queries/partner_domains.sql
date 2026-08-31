-- name: CreatePartnerDomain :one
INSERT INTO partner_domains (url, is_active, comment, legacy_kazik_domain_id)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListPartnerDomains :many
SELECT * FROM partner_domains ORDER BY created_at DESC;

-- name: ListActivePartnerDomains :many
SELECT * FROM partner_domains WHERE is_active = true ORDER BY created_at DESC;

-- name: GetPartnerDomainByID :one
SELECT * FROM partner_domains WHERE id = $1;

-- name: GetPartnerDomainByURL :one
SELECT * FROM partner_domains WHERE url = $1;

-- name: UpdatePartnerDomain :one
UPDATE partner_domains
SET url = COALESCE(sqlc.narg('url'), url),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    comment = COALESCE(sqlc.narg('comment'), comment),
    updated_at = now()
WHERE id = sqlc.arg('id') RETURNING *;

-- name: DeletePartnerDomain :exec
DELETE FROM partner_domains WHERE id = $1;

-- name: GetPartnerDomainByLegacyID :one
SELECT * FROM partner_domains WHERE legacy_kazik_domain_id = $1;
