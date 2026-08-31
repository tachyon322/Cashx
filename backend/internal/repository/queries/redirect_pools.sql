-- name: CreateRedirectPool :one
INSERT INTO redirect_pools (name, comment, legacy_kazik_redirect_id)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListRedirectPools :many
SELECT * FROM redirect_pools ORDER BY created_at DESC;

-- name: GetRedirectPoolByID :one
SELECT * FROM redirect_pools WHERE id = $1;

-- name: GetRedirectPoolByLegacyID :one
SELECT * FROM redirect_pools WHERE legacy_kazik_redirect_id = $1;

-- name: UpdateRedirectPool :one
UPDATE redirect_pools
SET name = COALESCE(sqlc.narg('name'), name),
    comment = COALESCE(sqlc.narg('comment'), comment),
    updated_at = now()
WHERE id = sqlc.arg('id') RETURNING *;

-- name: DeleteRedirectPool :exec
DELETE FROM redirect_pools WHERE id = $1;

-- name: CreateRedirectPoolURL :one
INSERT INTO redirect_pool_urls (redirect_id, url, weight, is_active, sort_order)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListRedirectPoolURLs :many
SELECT * FROM redirect_pool_urls WHERE redirect_id = $1 ORDER BY sort_order;

-- name: GetRedirectPoolURL :one
SELECT * FROM redirect_pool_urls WHERE id = $1 AND redirect_id = $2;

-- name: UpdateRedirectPoolURL :one
UPDATE redirect_pool_urls
SET url = COALESCE(sqlc.narg('url'), url),
    weight = COALESCE(sqlc.narg('weight'), weight),
    is_active = COALESCE(sqlc.narg('is_active'), is_active)
WHERE id = sqlc.arg('id') AND redirect_id = sqlc.arg('redirect_id') RETURNING *;

-- name: DeleteRedirectPoolURL :exec
DELETE FROM redirect_pool_urls WHERE id = $1 AND redirect_id = $2;

-- name: CountRedirectPoolURLs :one
SELECT count(*) FROM redirect_pool_urls WHERE redirect_id = $1;
