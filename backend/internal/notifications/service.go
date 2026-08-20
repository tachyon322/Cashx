// Package notifications creates in-app notifications for partners.
package notifications

import (
	"context"

	"cashx/internal/repository"
)

// NotifyUser inserts a personal notification row.
func NotifyUser(ctx context.Context, q *repository.Queries, userID, notifType, title, body string, metadata []byte) error {
	return q.InsertUserNotification(ctx, repository.InsertUserNotificationParams{
		UserID:   userID,
		Type:     notifType,
		Title:    title,
		Body:     body,
		Metadata: metadata,
	})
}
