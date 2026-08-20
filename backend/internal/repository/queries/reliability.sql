-- name: InsertOutboxMessage :one
INSERT INTO outbox_messages (topic, payload) VALUES ($1, $2) RETURNING id, topic, payload, status, attempts, next_attempt_at, last_error, created_at, sent_at;

-- name: ClaimOutboxMessages :many
SELECT id, topic, payload, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM outbox_messages
WHERE status = 'pending' AND next_attempt_at <= now()
ORDER BY id LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxSent :exec
UPDATE outbox_messages SET status = 'sent', sent_at = now(), last_error = NULL WHERE id = $1;

-- name: MarkOutboxFailed :exec
UPDATE outbox_messages
SET status = CASE WHEN attempts >= 10 THEN 'failed' ELSE 'pending' END,
    attempts = attempts + 1,
    next_attempt_at = now() + make_interval(mins => LEAST(60, power(2, attempts)::int)),
    last_error = $2
WHERE id = $1;

-- name: InsertAuditLog :exec
INSERT INTO audit_log (actor_user_id, action, entity_type, entity_id, changes, ip)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::inet);

-- name: ListAuditLog :many
SELECT l.id, l.actor_user_id, l.action, l.entity_type, l.entity_id, l.changes, l.ip, l.created_at,
       u.email AS actor_email
FROM audit_log l
LEFT JOIN users u ON u.id = l.actor_user_id
WHERE ($1 = '' OR l.entity_type = $1)
  AND ($2 = '' OR l.entity_id = $2)
ORDER BY l.created_at DESC LIMIT $3 OFFSET $4;

-- name: CountAuditLog :one
SELECT count(*) FROM audit_log
WHERE ($1 = '' OR entity_type = $1)
  AND ($2 = '' OR entity_id = $2);

-- name: GetPlatformSetting :one
SELECT key, value, updated_at FROM platform_settings WHERE key = $1;

-- name: SetPlatformSetting :one
INSERT INTO platform_settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
RETURNING key, value, updated_at;
