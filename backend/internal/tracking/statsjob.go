// statsjob.go — full recomputation of the daily aggregate tables
// (daily_partner_offer_stats, daily_tracking_link_stats) for a [from, to)
// window. Used by the worker (last 3 MSK days, every 60s) and by
// cmd/backfill-stats (whole history, e.g. after a kazik ETL run).
//
// Note on DeleteDailyStatsFrom(from): it deletes every aggregate row with
// day >= from, not just the window. Window passes must therefore run in
// ascending order (each later pass only rewrites its own month and later),
// which is how the backfill chunks months.
package tracking

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/repository"
)

// RecomputeDailyStats rewrites daily_partner_offer_stats and
// daily_tracking_link_stats for the [from, to) window from raw data
// (tracking_clicks, external_user_attributions, conversion_events,
// commission_earnings).
func RecomputeDailyStats(ctx context.Context, pool *pgxpool.Pool, from, to time.Time) error {
	return repository.WithTx(ctx, pool, func(q *repository.Queries) error {
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
