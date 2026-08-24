-- name: InsertAnnouncement :one
INSERT INTO announcements (title, body, audience, created_by)
VALUES ($1, $2, $3, $4) RETURNING id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at;

-- name: GetAnnouncement :one
SELECT id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at
FROM announcements WHERE id = $1;

-- name: UpdateAnnouncement :one
UPDATE announcements
SET title = COALESCE(sqlc.narg('title'), title),
    body = COALESCE(sqlc.narg('body'), body),
    audience = COALESCE(sqlc.narg('audience'), audience),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at;

-- name: PublishAnnouncement :one
UPDATE announcements SET is_published = true, published_at = now(), updated_at = now()
WHERE id = $1
RETURNING id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at;

-- name: SoftDeleteAnnouncement :exec
UPDATE announcements SET deleted_at = now(), updated_at = now() WHERE id = $1;

-- name: ListAnnouncements :many
SELECT id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at
FROM announcements ORDER BY created_at DESC;

-- name: ListVisibleAnnouncements :many
SELECT id, title, body, audience, is_published, published_at, created_by, created_at, updated_at, deleted_at
FROM announcements
WHERE is_published AND deleted_at IS NULL
ORDER BY published_at DESC LIMIT 100;

-- name: ReplaceAnnouncementAudiences :exec
DELETE FROM announcement_audiences WHERE announcement_id = $1;

-- name: InsertAnnouncementAudience :exec
INSERT INTO announcement_audiences (announcement_id, partner_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListAnnouncementAudiencePartnerIDs :many
SELECT partner_id FROM announcement_audiences WHERE announcement_id = $1;

-- name: InsertAnnouncementRead :exec
INSERT INTO announcement_reads (announcement_id, reader_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: GetAnnouncementRead :one
SELECT id, announcement_id, reader_user_id, read_at FROM announcement_reads WHERE announcement_id = $1 AND reader_user_id = $2;

-- name: MarkVisibleAnnouncementsRead :exec
INSERT INTO announcement_reads (announcement_id, reader_user_id)
SELECT a.id, $1::uuid FROM announcements a
WHERE a.is_published AND a.deleted_at IS NULL
  AND (a.audience IN ('all', 'partners', 'staff') OR
       EXISTS (SELECT 1 FROM announcement_audiences aa WHERE aa.announcement_id = a.id AND aa.partner_id = $2::uuid))
ON CONFLICT DO NOTHING;

-- name: InsertUserNotification :exec
INSERT INTO user_notifications (user_id, type, title, body, metadata)
VALUES ($1, $2, $3, $4, $5);

-- name: ListUserNotifications :many
SELECT id, user_id, type, title, body, metadata, read_at, created_at
FROM user_notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: CountUnreadUserNotifications :one
SELECT count(*) FROM user_notifications WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkUserNotificationRead :exec
UPDATE user_notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkAllUserNotificationsRead :exec
UPDATE user_notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;
