package integration_test

import (
	"testing"
	"time"

	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// A partner joined to offers of two projects has two daily_partner_offer_stats
// rows per day; the dashboard chart must sum them, not keep one at random.
func TestDailySumsOffersOfSameDay(t *testing.T) {
	pool := setup(t)
	data, _, _ := fullSetup(t, pool)
	ctx := t.Context()

	var secondOffer string
	if err := adminDB.QueryRowContext(ctx, `
		WITH p AS (
			INSERT INTO projects (slug, name, destination_url)
			VALUES ('second-project', 'Second', 'https://second.example')
			RETURNING id
		)
		INSERT INTO offers (project_id, name, status)
		SELECT id, 'Second offer', 'active' FROM p
		RETURNING id`).Scan(&secondOffer); err != nil {
		t.Fatalf("second offer: %v", err)
	}

	day := tracking.StartOfMSKDay(time.Now())
	if _, err := adminDB.ExecContext(ctx, `
		INSERT INTO daily_partner_offer_stats (partner_id, offer_id, day, clicks, unique_clicks, registrations, first_payments, income_kopecks)
		VALUES ($1, $2, $4, 321, 200, 60, 10, 2300000),
		       ($1, $3, $4, 1, 1, 0, 0, 0)`,
		data.PartnerID, data.Offer, secondOffer, day.Format("2006-01-02")); err != nil {
		t.Fatalf("insert daily stats: %v", err)
	}

	stats, err := tracking.Daily(ctx, repository.New(pool), data.PartnerID, tracking.Today())
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("want 1 day, got %d: %+v", len(stats), stats)
	}
	want := tracking.DayStats{
		Date: day.Format("2006-01-02"), Clicks: 322, UniqueClicks: 201,
		Registrations: 60, FirstPayments: 10, IncomeKopecks: 2300000,
	}
	if stats[0] != want {
		t.Fatalf("daily: want %+v, got %+v", want, stats[0])
	}
}
