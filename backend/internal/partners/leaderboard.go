package partners

import (
	"context"
	"sort"
	"time"

	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// LeaderboardEntry is one row of the leaderboard.
type LeaderboardEntry struct {
	PartnerID          string  `json:"partner_id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	RevsharePercentBps int     `json:"revshare_percent_bps"`
	Clicks             int64   `json:"clicks"`
	Signups            int64   `json:"signups"`
	Promos             int64   `json:"promos"`
	Depositors         int64   `json:"depositors"`
	DepositsSum        int64   `json:"deposits_sum"`
	Income             int64   `json:"income"`
	Cr                 *float64 `json:"cr"`
}

// GetLeaderboard returns top partners sorted by metric.
func (s *Service) GetLeaderboard(ctx context.Context, period, metric string) ([]LeaderboardEntry, error) {
	// Normalize period
	var from *time.Time
	now := time.Now()
	switch period {
	case "week":
		t := tracking.StartOfMSKDay(now).AddDate(0, 0, -6)
		from = &t
	case "month":
		t := tracking.StartOfMSKDay(now).AddDate(0, 0, -29)
		from = &t
	default:
		period = "all"
	}
	if metric != "clicks" && metric != "signups" && metric != "deposits" && metric != "income" {
		metric = "income"
	}
	// List approved partners
	rows, err := s.Pool.Query(ctx, `SELECT p.id, u.name, u.email, p.revshare_percent_bps FROM partner_profiles p JOIN users u ON u.id=p.user_id WHERE p.is_approved=true AND p.is_blocked=false ORDER BY p.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		var revshare int32
		if err := rows.Scan(&e.PartnerID, &e.Name, &e.Email, &revshare); err != nil {
			continue
		}
		e.RevsharePercentBps = int(revshare)
		// Get link IDs for this partner
		linkRows, err := s.Pool.Query(ctx, `SELECT tl.id FROM tracking_links tl JOIN partner_offer_accesses a ON a.id=tl.partner_offer_access_id WHERE a.partner_id=$1`, e.PartnerID)
		if err != nil {
			continue
		}
		var linkIDs []string
		for linkRows.Next() {
			var id string
			_ = linkRows.Scan(&id)
			linkIDs = append(linkIDs, id)
		}
		linkRows.Close()
		var r tracking.Range
		if from != nil {
			r.From = from
		}
		aggMap, _ := tracking.AggregateForSources(ctx, repository.New(s.Pool), tracking.DefaultCounters, linkIDs, r)
		total := tracking.TotalAggregate(aggMap)
		e.Clicks = total.Clicks
		e.Signups = total.Signups
		e.Promos = total.Promos
		e.Depositors = total.Depositors
		e.DepositsSum = total.DepositsSum
		e.Income = total.Income
		e.Cr = total.Cr
		entries = append(entries, e)
	}
	// Sort
	sort.Slice(entries, func(i, j int) bool {
		var vi, vj int64
		switch metric {
		case "clicks":
			vi, vj = entries[i].Clicks, entries[j].Clicks
		case "signups":
			vi, vj = entries[i].Signups, entries[j].Signups
		case "deposits":
			vi, vj = entries[i].DepositsSum, entries[j].DepositsSum
		default: // income
			vi, vj = entries[i].Income, entries[j].Income
		}
		if vi != vj {
			return vj < vi
		}
		if entries[i].DepositsSum != entries[j].DepositsSum {
			return entries[j].DepositsSum < entries[i].DepositsSum
		}
		if entries[i].Clicks != entries[j].Clicks {
			return entries[j].Clicks < entries[i].Clicks
		}
		if entries[i].Signups != entries[j].Signups {
			return entries[j].Signups < entries[i].Signups
		}
		return entries[i].Name < entries[j].Name
	})
	if len(entries) > 100 {
		entries = entries[:100]
	}
	return entries, nil
}
