// Package worker implements the background worker: outbox delivery and daily
// statistics aggregation.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/outbox"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// Runner owns the worker loops.
type Runner struct {
	Pool   *pgxpool.Pool
	Cfg    platform.Config
	Log    *slog.Logger
	Registry outbox.Registry
}

// New builds the worker.
func New(pool *pgxpool.Pool, cfg platform.Config, log *slog.Logger) *Runner {
	return &Runner{Pool: pool, Cfg: cfg, Log: log, Registry: outbox.NewRegistry(cfg, log)}
}

// Run starts both loops until ctx is done.
func (r *Runner) Run(ctx context.Context) {
	go r.outboxLoop(ctx)
	r.statsLoop(ctx)
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

// recomputeStats rewrites daily_partner_offer_stats for the current and
// previous two MSK days.
func (r *Runner) recomputeStats(ctx context.Context) error {
	now := time.Now()
	from := tracking.StartOfMSKDay(now).AddDate(0, 0, -2)
	to := tracking.StartOfMSKDay(now).AddDate(0, 0, 1) // exclusive

	return repository.WithTx(ctx, r.Pool, func(q *repository.Queries) error {
		if err := q.DeleteDailyStatsFrom(ctx, repository.DatePtr(from)); err != nil {
			return err
		}
		clicks, err := q.AggClicksByDay(ctx, repository.AggClicksByDayParams{CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to)})
		if err != nil {
			return err
		}
		uniqueClicks, err := q.AggUniqueClicksByDay(ctx, repository.AggUniqueClicksByDayParams{CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to)})
		if err != nil {
			return err
		}
		regs, err := q.AggRegistrationsByDay(ctx, repository.AggRegistrationsByDayParams{FirstSeenAt: repository.TimePtr(&from), FirstSeenAt_2: repository.TimePtr(&to)})
		if err != nil {
			return err
		}
		firsts, err := q.AggFirstPaymentsByDay(ctx, repository.AggFirstPaymentsByDayParams{OccurredAt: repository.TimePtr(&from), OccurredAt_2: repository.TimePtr(&to)})
		if err != nil {
			return err
		}
		income, err := q.AggIncomeByDay(ctx, repository.AggIncomeByDayParams{CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to)})
		if err != nil {
			return err
		}
		type key struct {
			partner, offer string
			day            time.Time
		}
		merged := make(map[key]*repository.UpsertDailyStatsParams)
		add := func(partner, offer string, day time.Time) *repository.UpsertDailyStatsParams {
			k := key{partner, offer, day}
			if e, ok := merged[k]; ok {
				return e
			}
			e := &repository.UpsertDailyStatsParams{PartnerID: partner, OfferID: offer, Day: repository.DatePtr(day)}
			merged[k] = e
			return e
		}
		for _, c := range clicks {
			add(c.PartnerID, c.OfferID, c.Day.Time).Clicks = int32(c.Clicks)
		}
		for _, uc := range uniqueClicks {
			add(uc.PartnerID, uc.OfferID, uc.Day.Time).UniqueClicks = int32(uc.UniqueClicks)
		}
		for _, rg := range regs {
			add(*repository.UUIDToPtr(rg.PartnerID), *repository.UUIDToPtr(rg.OfferID), rg.Day.Time).Registrations = int32(rg.Registrations)
		}
		for _, f := range firsts {
			add(*repository.UUIDToPtr(f.PartnerID), *repository.UUIDToPtr(f.OfferID), f.Day.Time).FirstPayments = int32(f.FirstPayments)
		}
		for _, inc := range income {
			add(inc.PartnerID, inc.OfferID, inc.Day.Time).IncomeKopecks = inc.IncomeKopecks
		}
		for _, p := range merged {
			if _, err := q.UpsertDailyStats(ctx, *p); err != nil {
				return err
			}
		}

		// Per-source (tracking link) stats for the same window.
		if err := q.DeleteDailyLinkStatsFrom(ctx, repository.DatePtr(from)); err != nil {
			return err
		}
		linkClicks, err := q.AggClicksByLink(ctx, repository.AggClicksByLinkParams{
			CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to),
		})
		if err != nil {
			return err
		}
		linkUnique, err := q.AggUniqueClicksByLink(ctx, repository.AggUniqueClicksByLinkParams{
			CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to),
		})
		if err != nil {
			return err
		}
		linkRegs, err := q.AggRegistrationsByLink(ctx, repository.AggRegistrationsByLinkParams{
			FirstSeenAt: repository.TimePtr(&from), FirstSeenAt_2: repository.TimePtr(&to),
		})
		if err != nil {
			return err
		}
		linkFirsts, err := q.AggFirstPaymentsByLink(ctx, repository.AggFirstPaymentsByLinkParams{
			OccurredAt: repository.TimePtr(&from), OccurredAt_2: repository.TimePtr(&to),
		})
		if err != nil {
			return err
		}
		linkIncome, err := q.AggIncomeByLink(ctx, repository.AggIncomeByLinkParams{
			CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to),
		})
		if err != nil {
			return err
		}
		type linkKey struct {
			link string
			day  time.Time
		}
		linkMerged := make(map[linkKey]*repository.UpsertDailyLinkStatsParams)
		addLink := func(link string, day time.Time) *repository.UpsertDailyLinkStatsParams {
			k := linkKey{link, day}
			if e, ok := linkMerged[k]; ok {
				return e
			}
			e := &repository.UpsertDailyLinkStatsParams{TrackingLinkID: link, Day: repository.DatePtr(day)}
			linkMerged[k] = e
			return e
		}
		for _, c := range linkClicks {
			addLink(c.TrackingLinkID, c.Day.Time).Clicks = int32(c.Clicks)
		}
		for _, uc := range linkUnique {
			addLink(uc.TrackingLinkID, uc.Day.Time).UniqueClicks = int32(uc.UniqueClicks)
		}
		for _, rg := range linkRegs {
			addLink(rg.TrackingLinkID, rg.Day.Time).Registrations = int32(rg.Registrations)
		}
		for _, f := range linkFirsts {
			addLink(f.TrackingLinkID, f.Day.Time).FirstPayments = int32(f.FirstPayments)
		}
		for _, inc := range linkIncome {
			if !inc.TrackingLinkID.Valid {
				continue
			}
			addLink(inc.TrackingLinkID.String(), inc.Day.Time).IncomeKopecks = inc.IncomeKopecks
		}
		for _, p := range linkMerged {
			if _, err := q.UpsertDailyLinkStats(ctx, *p); err != nil {
				return err
			}
		}
		return nil
	})
}
