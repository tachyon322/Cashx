// Package worker implements the background worker: outbox delivery and daily
// statistics aggregation.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"cashx/internal/outbox"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// Runner owns the worker loops.
type Runner struct {
	Pool     *pgxpool.Pool
	Cfg      platform.Config
	Log      *slog.Logger
	Redis    *redis.Client
	Counters *tracking.Counters
	Registry outbox.Registry
}

// New builds the worker.
func New(pool *pgxpool.Pool, cfg platform.Config, log *slog.Logger) *Runner {
	return &Runner{Pool: pool, Cfg: cfg, Log: log, Registry: outbox.NewRegistry(cfg, log)}
}

// NewWithRedis builds the worker with Redis counters.
func NewWithRedis(pool *pgxpool.Pool, cfg platform.Config, log *slog.Logger, rdb *redis.Client) *Runner {
	c := tracking.NewCounters(rdb)
	tracking.DefaultCounters = c
	return &Runner{Pool: pool, Cfg: cfg, Log: log, Redis: rdb, Counters: c, Registry: outbox.NewRegistry(cfg, log)}
}

// Run starts all loops until ctx is done.
func (r *Runner) Run(ctx context.Context) {
	go r.outboxLoop(ctx)
	go r.countersLoop(ctx)
	go r.syncPayoutLoop(ctx)
	go r.reconcileLoop(ctx)
	go r.partitionsLoop(ctx)
	r.statsLoop(ctx)
}

// partitionsLoop keeps monthly partitions of tracking_clicks /
// incoming_events / conversion_events ahead of time: 36 months back (covers
// replays/backdated imports) and 2 months ahead. A BEFORE STATEMENT trigger
// (see migrations/00021_partition_security.sql) additionally guards the
// months around now on every INSERT, so a month rollover can never fail
// live ingestion again.
func (r *Runner) partitionsLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	if err := r.ensurePartitions(ctx); err != nil {
		r.Log.Error("ensure partitions", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.ensurePartitions(ctx); err != nil {
				r.Log.Error("ensure partitions", "err", err)
			}
		}
	}
}

func (r *Runner) ensurePartitions(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, "SELECT ensure_partitions_range(36, 2)")
	return err
}

// outboxLoop delivers pending outbox messages every 15s.
func (r *Runner) outboxLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.deliverOnce(ctx); err != nil {
				r.Log.Error("outbox delivery", "err", err)
			}
		}
	}
}

func (r *Runner) deliverOnce(ctx context.Context) error {
	q := repository.New(r.Pool)
	msgs, err := outbox.Claim(ctx, q, 100)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		d, ok := r.Registry[m.Topic]
		if !ok {
			r.Log.Warn("no dispatcher for outbox topic; dropping", "topic", m.Topic, "id", m.ID)
			_ = outbox.MarkSent(ctx, q, m.ID)
			continue
		}
		if err := d.Dispatch(ctx, m); err != nil {
			r.Log.Error("dispatch failed", "topic", m.Topic, "id", m.ID, "err", err)
			_ = outbox.MarkFailed(ctx, q, m.ID, err.Error())
			continue
		}
		_ = outbox.MarkSent(ctx, q, m.ID)
	}
	return nil
}

// statsLoop recomputes daily partner/offer stats every 60s for the last 3 MSK
// days, keeping the cabinet charts fresh even without scheduled runs.
func (r *Runner) statsLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	// First run immediately so dev data appears without waiting.
	if err := r.recomputeStats(ctx); err != nil {
		r.Log.Error("recompute stats", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.recomputeStats(ctx); err != nil {
				r.Log.Error("recompute stats", "err", err)
			}
		}
	}
}

// recomputeStats rewrites daily stats for the current and previous two MSK
// days (the shared implementation lives in tracking.RecomputeDailyStats and
// is also used by cmd/backfill-stats for whole-history runs).
func (r *Runner) recomputeStats(ctx context.Context) error {
	now := time.Now()
	from := tracking.StartOfMSKDay(now).AddDate(0, 0, -2)
	to := tracking.StartOfMSKDay(now).AddDate(0, 0, 1) // exclusive
	return tracking.RecomputeDailyStats(ctx, r.Pool, from, to)
}

// countersLoop flushes Redis click buffer every 5s.
func (r *Runner) countersLoop(ctx context.Context) {
	if r.Counters == nil || r.Redis == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Counters.Flush(ctx, func(ctx context.Context, linkID, ip, ua, ref string, at time.Time) error {
				q := repository.New(r.Pool)
				params := repository.CreateTrackingClickParams{
					TrackingLinkID: linkID,
					Column2:        ip,
					UserAgent:      repository.TextPtr(&ua),
					Referrer:       repository.TextPtr(&ref),
				}
				return tracking.InsertWithPartitionRetry(ctx, r.Pool, at, func() error {
					_, err := q.CreateTrackingClick(ctx, params)
					return err
				})
			}); err != nil {
				r.Log.Error("counters flush", "err", err)
			}
		}
	}
}

// syncPayoutLoop syncs payout_rules to Redis every 5m (for kazik parity).
func (r *Runner) syncPayoutLoop(ctx context.Context) {
	if r.Redis == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	// Run once at start
	_ = r.syncPayoutOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.syncPayoutOnce(ctx); err != nil {
				r.Log.Error("sync payout_rules", "err", err)
			}
		}
	}
}

func (r *Runner) syncPayoutOnce(ctx context.Context) error {
	q := repository.New(r.Pool)
	row, err := q.GetPayoutRules(ctx)
	if err != nil {
		return err
	}
	pipe := r.Redis.Pipeline()
	pipe.Set(ctx, "affiliate:usdt_rate", fmt.Sprintf("%v", row.UsdtRate), 0)
	pipe.Set(ctx, "affiliate:sbp_fee_flat", row.SbpFeeFlatKopecks, 0)
	pipe.Set(ctx, "affiliate:sbp_fee_percent", row.SbpFeePercentBps, 0)
	pipe.Set(ctx, "affiliate:min_withdraw", row.MinWithdrawKopecks, 0)
	_, err = pipe.Exec(ctx)
	return err
}

// reconcileLoop checks drift hourly (stub, logs if >1% diff).
func (r *Runner) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simple check: compare tracking_clicks last hour vs Redis stats
			// For now just log that reconcile ran.
			r.Log.Info("reconcile tick")
		}
	}
}
