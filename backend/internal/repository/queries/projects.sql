-- name: CreateProject :one
INSERT INTO projects (slug, name, description, destination_url, is_active)
VALUES ($1, $2, $3, $4, $5) RETURNING id, slug, name, description, destination_url, is_active, created_at, updated_at;

-- name: GetProjectBySlug :one
SELECT id, slug, name, description, destination_url, is_active, created_at, updated_at FROM projects WHERE slug = $1;

-- name: GetProjectByID :one
SELECT id, slug, name, description, destination_url, is_active, created_at, updated_at FROM projects WHERE id = $1;

-- name: UpdateProject :one
UPDATE projects
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    destination_url = COALESCE(sqlc.narg('destination_url'), destination_url),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, slug, name, description, destination_url, is_active, created_at, updated_at;

-- name: ListProjects :many
SELECT id, slug, name, description, destination_url, is_active, created_at, updated_at
FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountProjects :one
SELECT count(*) FROM projects;

-- name: UpsertProjectSettings :exec
INSERT INTO project_settings (project_id) VALUES ($1) ON CONFLICT (project_id) DO NOTHING;

-- name: GetProjectSettings :one
SELECT project_id, allow_new_partners, comment, created_at, updated_at FROM project_settings WHERE project_id = $1;

-- name: CreateIntegrationKey :one
INSERT INTO integration_keys (project_id, key_id, secret_ciphertext, secret_hint)
VALUES ($1, $2, $3, $4) RETURNING id, project_id, key_id, is_active, created_at, rotated_at, last_used_at, secret_hint;

-- name: GetIntegrationKeyByKeyID :one
SELECT id, project_id, key_id, secret_ciphertext, secret_hint, is_active, created_at, rotated_at, last_used_at
FROM integration_keys WHERE key_id = $1;

-- name: ListIntegrationKeysByProject :many
SELECT id, project_id, key_id, is_active, created_at, rotated_at, last_used_at, secret_hint
FROM integration_keys WHERE project_id = $1 ORDER BY created_at DESC;

-- name: DeactivateIntegrationKey :exec
UPDATE integration_keys SET is_active = false, rotated_at = COALESCE(rotated_at, now()) WHERE key_id = $1;

-- name: TouchIntegrationKey :exec
UPDATE integration_keys SET last_used_at = now() WHERE key_id = $1;
