package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx runs fn inside a transaction bound to a fresh Queries set.
// The transaction is committed only when fn returns nil.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
