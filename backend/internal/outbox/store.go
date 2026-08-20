// Package outbox implements a PostgreSQL-backed reliable outbox queue consumed
// by the worker.
package outbox

import (
	"context"
	"encoding/json"

	"cashx/internal/repository"
)

// Enqueue appends a message to the outbox.
func Enqueue(ctx context.Context, q *repository.Queries, topic string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = q.InsertOutboxMessage(ctx, repository.InsertOutboxMessageParams{Topic: topic, Payload: raw})
	return err
}

// Message is a claimed outbox row.
type Message = repository.OutboxMessage

// Claim atomically takes up to limit pending messages that are due.
func Claim(ctx context.Context, q *repository.Queries, limit int) ([]Message, error) {
	return q.ClaimOutboxMessages(ctx, int32(limit))
}

// MarkSent marks a message as delivered.
func MarkSent(ctx context.Context, q *repository.Queries, id int64) error {
	return q.MarkOutboxSent(ctx, id)
}

// MarkFailed records a delivery error and schedules the retry with backoff
// (2^attempts minutes, capped at 60; after 10 attempts the message fails).
func MarkFailed(ctx context.Context, q *repository.Queries, id int64, errMsg string) error {
	return q.MarkOutboxFailed(ctx, repository.MarkOutboxFailedParams{ID: id, LastError: repository.TextPtr(&errMsg)})
}
