package tracking

import (
	"context"
	"time"

	"cashx/internal/repository"
)

// DayStats is one day of aggregated statistics.
type DayStats struct {
	Date          string `json:"date"`
	Clicks        int64  `json:"clicks"`
	UniqueClicks  int64  `json:"unique_clicks"`
	Registrations int64  `json:"registrations"`
	FirstPayments int64  `json:"first_payments"`
	IncomeKopecks int64  `json:"income_kopecks"`
}

// Totals is an aggregate over a period.
type Totals struct {
	Clicks        int64 `json:"clicks"`
	UniqueClicks  int64 `json:"unique_clicks"`
	Registrations int64 `json:"registrations"`
	FirstPayments int64 `json:"first_payments"`
	IncomeKopecks int64 `json:"income_kopecks"`
}

// Period bounds in MSK days, inclusive.
type Period struct {
	From time.Time // MSK start of first day
	To   time.Time // MSK end of last day
}

// Today/Week/Month return inclusive MSK periods.
func Today() Period {
	start := StartOfMSKDay(time.Now())
	return Period{From: start, To: start.Add(24 * time.Hour).Add(-time.Second)}
}

func LastDays(n int) Period {
	start := StartOfMSKDay(time.Now()).AddDate(0, 0, -(n - 1))
	return Period{From: start, To: StartOfMSKDay(time.Now()).Add(24 * time.Hour).Add(-time.Second)}
}

func AllTime() Period {
	return Period{From: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Now().Add(365 * 24 * time.Hour)}
}

// Daily returns per-day stats for the partner over the period, filling gaps
// with zero rows.
func Daily(ctx context.Context, q *repository.Queries, partnerID string, p Period) ([]DayStats, error) {
	rows, err := q.GetDailyStats(ctx, repository.GetDailyStatsParams{
		PartnerID: partnerID, Day: repository.DatePtr(p.From), Day_2: repository.DatePtr(p.To),
	})
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]DayStats, len(rows))
	for _, r := range rows {
		day := r.Day.Time.In(MSK).Format("2006-01-02")
		byDay[day] = DayStats{
			Date: day, Clicks: int64(r.Clicks), UniqueClicks: int64(r.UniqueClicks),
			Registrations: int64(r.Registrations),
			FirstPayments: int64(r.FirstPayments), IncomeKopecks: r.IncomeKopecks,
		}
	}
	out := make([]DayStats, 0, 31)
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		key := d.In(MSK).Format("2006-01-02")
		if v, ok := byDay[key]; ok {
			out = append(out, v)
		} else {
			out = append(out, DayStats{Date: key})
		}
	}
	return out, nil
}

// DailyByOffer is like Daily but scoped to one offer.
func DailyByOffer(ctx context.Context, q *repository.Queries, partnerID, offerID string, p Period) ([]DayStats, error) {
	rows, err := q.GetDailyStatsByOffer(ctx, repository.GetDailyStatsByOfferParams{
		PartnerID: partnerID, OfferID: offerID,
		Day: repository.DatePtr(p.From), Day_2: repository.DatePtr(p.To),
	})
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]DayStats, len(rows))
	for _, r := range rows {
		day := r.Day.Time.In(MSK).Format("2006-01-02")
		byDay[day] = DayStats{
			Date: day, Clicks: int64(r.Clicks), UniqueClicks: int64(r.UniqueClicks),
			Registrations: int64(r.Registrations),
			FirstPayments: int64(r.FirstPayments), IncomeKopecks: r.IncomeKopecks,
		}
	}
	out := make([]DayStats, 0, 31)
	for d := p.From; !d.After(p.To); d = d.AddDate(0, 0, 1) {
		key := d.In(MSK).Format("2006-01-02")
		if v, ok := byDay[key]; ok {
			out = append(out, v)
		} else {
			out = append(out, DayStats{Date: key})
		}
	}
	return out, nil
}

// TotalsFor sums stats over a period (partner-wide).
func TotalsFor(ctx context.Context, q *repository.Queries, partnerID string, p Period) (Totals, error) {
	if p.To.Before(p.From) {
		return Totals{}, nil
	}
	row, err := q.SumDailyStats(ctx, repository.SumDailyStatsParams{
		PartnerID: partnerID, Day: repository.DatePtr(p.From), Day_2: repository.DatePtr(p.To),
	})
	if err != nil {
		return Totals{}, err
	}
	return Totals{Clicks: row.Clicks, UniqueClicks: row.UniqueClicks, Registrations: row.Registrations, FirstPayments: row.FirstPayments, IncomeKopecks: row.IncomeKopecks}, nil
}

// TotalsAllTime sums all partner stats.
func TotalsAllTime(ctx context.Context, q *repository.Queries, partnerID string) (Totals, error) {
	row, err := q.SumDailyStatsAllTime(ctx, partnerID)
	if err != nil {
		return Totals{}, err
	}
	return Totals{Clicks: row.Clicks, UniqueClicks: row.UniqueClicks, Registrations: row.Registrations, FirstPayments: row.FirstPayments, IncomeKopecks: row.IncomeKopecks}, nil
}

// TotalsOffer sums stats for one offer over a period.
func TotalsOffer(ctx context.Context, q *repository.Queries, partnerID, offerID string, p Period) (Totals, error) {
	row, err := q.SumDailyStatsByOffer(ctx, repository.SumDailyStatsByOfferParams{
		PartnerID: partnerID, OfferID: offerID,
		Day: repository.DatePtr(p.From), Day_2: repository.DatePtr(p.To),
	})
	if err != nil {
		return Totals{}, err
	}
	return Totals{Clicks: row.Clicks, UniqueClicks: row.UniqueClicks, Registrations: row.Registrations, FirstPayments: row.FirstPayments, IncomeKopecks: row.IncomeKopecks}, nil
}

// TotalsOfferAllTime sums all stats for one offer.
func TotalsOfferAllTime(ctx context.Context, q *repository.Queries, partnerID, offerID string) (Totals, error) {
	row, err := q.SumDailyStatsOfferAllTime(ctx, repository.SumDailyStatsOfferAllTimeParams{PartnerID: partnerID, OfferID: offerID})
	if err != nil {
		return Totals{}, err
	}
	return Totals{Clicks: row.Clicks, UniqueClicks: row.UniqueClicks, Registrations: row.Registrations, FirstPayments: row.FirstPayments, IncomeKopecks: row.IncomeKopecks}, nil
}

// --- Phase 2.5 extensions ---

// StatsAggregate mirrors kazik's SourceStatsAggregate.
type StatsAggregate struct {
	Clicks        int64    `json:"clicks"`
	UniqueClicks  int64    `json:"unique_clicks"`
	Signups       int64    `json:"signups"`
	Promos        int64    `json:"promos"`
	Depositors    int64    `json:"depositors"`
	DepositsCount int64    `json:"deposits_count"`
	DepositsSum   int64    `json:"deposits_sum"`
	Income        int64    `json:"income"`
	Cr            *float64 `json:"cr"`
	CrPayment     *float64 `json:"cr_payment"`
}

// Range is like Period but nullable for stats filtering.
type Range struct {
	From *time.Time
	To   *time.Time
}

// ParseRange parses from/to strings (YYYY-MM-DD or RFC3339) in MSK.
func ParseRange(fromStr, toStr string) Range {
	var r Range
	if fromStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", fromStr, MSK); err == nil {
			tt := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, MSK)
			r.From = &tt
		} else if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			r.From = &t
		}
	}
	if toStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", toStr, MSK); err == nil {
			tt := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, MSK)
			r.To = &tt
		} else if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			r.To = &t
		}
	}
	return r
}

// AggregateForSources aggregates per-link stats, preferring Redis counters if available.
func AggregateForSources(ctx context.Context, q *repository.Queries, counters *Counters, linkIDs []string, r Range) (map[string]StatsAggregate, error) {
	if len(linkIDs) == 0 {
		return map[string]StatsAggregate{}, nil
	}
	// Try Redis first if no date filter
	if counters != nil && r.From == nil && r.To == nil {
		if m, ok := counters.GetStats(ctx, linkIDs); ok {
			// Convert to StatsAggregate (already)
			out := make(map[string]StatsAggregate, len(m))
			for k, v := range m {
				agg := v
				if agg.Clicks > 0 {
					cr := float64(agg.Signups) / float64(agg.Clicks) * 100
					agg.Cr = &cr
					crPay := float64(agg.Depositors) / float64(agg.Clicks) * 100
					agg.CrPayment = &crPay
				}
				out[k] = agg
			}
			return out, nil
		}
	}
	// Fallback PG: query per link
	out := make(map[string]StatsAggregate, len(linkIDs))
	for _, id := range linkIDs {
		var from, to *time.Time
		if r.From != nil {
			from = r.From
		}
		if r.To != nil {
			to = r.To
		}
		// Use existing daily stats if range, else all-time
		var agg StatsAggregate
		if from != nil || to != nil {
			// For range, use daily stats sum if available, else raw
			// Simplified: use SumDailyLinkStats
			var row struct {
				Clicks        int64
				UniqueClicks  int64
				Registrations int64
				FirstPayments int64
				IncomeKopecks int64
			}
			// Try daily_link_stats first
			if from != nil && to != nil {
				if res, err := q.SumDailyLinkStats(ctx, repository.SumDailyLinkStatsParams{TrackingLinkID: id, Day: repository.DatePtr(*from), Day_2: repository.DatePtr(*to)}); err == nil {
					row.Clicks = res.Clicks
					row.UniqueClicks = res.UniqueClicks
					row.Registrations = res.Registrations
					row.FirstPayments = res.FirstPayments
					row.IncomeKopecks = res.IncomeKopecks
				}
			} else {
				if res, err := q.SumDailyLinkStatsAllTime(ctx, id); err == nil {
					row.Clicks = res.Clicks
					row.UniqueClicks = res.UniqueClicks
					row.Registrations = res.Registrations
					row.FirstPayments = res.FirstPayments
					row.IncomeKopecks = res.IncomeKopecks
				}
			}
			agg.Clicks = row.Clicks
			agg.UniqueClicks = row.UniqueClicks
			agg.Signups = row.Registrations
			agg.Depositors = row.FirstPayments
			agg.Income = row.IncomeKopecks
			agg.DepositsSum = row.IncomeKopecks // approximation
		} else {
			if res, err := q.SumDailyLinkStatsAllTime(ctx, id); err == nil {
				agg.Clicks = res.Clicks
				agg.UniqueClicks = res.UniqueClicks
				agg.Signups = res.Registrations
				agg.Depositors = res.FirstPayments
				agg.Income = res.IncomeKopecks
				agg.DepositsSum = res.IncomeKopecks
			}
		}
		// Count promos separately
		if link, err := q.GetTrackingLinkByID(ctx, id); err == nil && link.Type == "promo" {
			agg.Promos = agg.Signups
			agg.Signups = 0
		}
		if agg.Clicks > 0 {
			cr := float64(agg.Signups) / float64(agg.Clicks) * 100
			agg.Cr = &cr
			crPay := float64(agg.Depositors) / float64(agg.Clicks) * 100
			agg.CrPayment = &crPay
		}
		out[id] = agg
	}
	return out, nil
}

// TotalAggregate sums a map of aggregates.
func TotalAggregate(m map[string]StatsAggregate) StatsAggregate {
	var total StatsAggregate
	for _, v := range m {
		total.Clicks += v.Clicks
		total.UniqueClicks += v.UniqueClicks
		total.Signups += v.Signups
		total.Promos += v.Promos
		total.Depositors += v.Depositors
		total.DepositsCount += v.DepositsCount
		total.DepositsSum += v.DepositsSum
		total.Income += v.Income
	}
	if total.Clicks > 0 {
		cr := float64(total.Signups) / float64(total.Clicks) * 100
		total.Cr = &cr
		crPay := float64(total.Depositors) / float64(total.Clicks) * 100
		total.CrPayment = &crPay
	}
	return total
}

// TopSources returns up to limit linkIDs sorted by income.
func TopSources(agg map[string]StatsAggregate, meta map[string]string, limit int) []string {
	type kv struct {
		ID     string
		Income int64
		Clicks int64
	}
	var list []kv
	for id, a := range agg {
		list = append(list, kv{ID: id, Income: a.Income, Clicks: a.Clicks})
	}
	// sort by income desc, then clicks
	// Use simple sort
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].Income > list[i].Income || (list[j].Income == list[i].Income && list[j].Clicks > list[i].Clicks) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	if len(list) > limit {
		list = list[:limit]
	}
	ids := make([]string, 0, len(list))
	for _, kv := range list {
		ids = append(ids, kv.ID)
	}
	return ids
}
