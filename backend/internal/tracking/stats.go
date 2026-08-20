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
