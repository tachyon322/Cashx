-- name: CreateOfferDomain :one
INSERT INTO offer_domains (offer_id, url, is_main, is_active, comment)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListOfferDomains :many
SELECT * FROM offer_domains WHERE offer_id = $1 ORDER BY is_main DESC, created_at;

-- name: ListActiveOfferDomains :many
SELECT * FROM offer_domains WHERE offer_id = $1 AND is_active = true ORDER BY is_main DESC, created_at;

-- name: GetOfferDomainByID :one
SELECT * FROM offer_domains WHERE id = $1;

-- name: GetOfferDomainByOfferAndID :one
SELECT * FROM offer_domains WHERE id = $1 AND offer_id = $2;

-- name: GetMainOfferDomain :one
SELECT * FROM offer_domains WHERE offer_id = $1 AND is_main = true AND is_active = true LIMIT 1;

-- name: ClearMainOfferDomain :exec
UPDATE offer_domains SET is_main = false, updated_at = now()
WHERE offer_id = $1 AND is_main = true;

-- name: SetOfferDomainMain :exec
UPDATE offer_domains SET is_main = $2, updated_at = now() WHERE id = $1;

-- name: UpdateOfferDomain :one
UPDATE offer_domains
SET url = COALESCE(sqlc.narg('url'), url),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    comment = COALESCE(sqlc.narg('comment'), comment),
    updated_at = now()
WHERE id = sqlc.arg('id') RETURNING *;

-- name: DeleteOfferDomain :exec
DELETE FROM offer_domains WHERE id = $1;

-- name: CountOfferDomains :one
SELECT count(*) FROM offer_domains WHERE offer_id = $1;
