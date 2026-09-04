package partners

import (
	"context"
	"fmt"
	"time"
)

// B2CReferralItem mirrors kazik's ReferralItem for players.
type B2CReferralItem struct {
	UserID            string  `json:"user_id"`
	Name              string  `json:"name"`
	Email             *string `json:"email"`
	Kind              string  `json:"kind"` // registration|promo
	CreatedAt         string  `json:"created_at"`
	SourceID          string  `json:"source_id"`
	SourceName        string  `json:"source_name"`
	DepositsCount     int64   `json:"deposits_count"`
	DepositsSum       int64   `json:"deposits_sum"`       // kopecks
	Income            int64   `json:"income"`             // kopecks, commission
	CommissionPercent int     `json:"commission_percent"` // 0-100
}

// GetB2CReferrals returns player referrals (B2C) for a partner, deduped by
// external_user_id (first-touch: the earliest attribution wins).
// Range filters by first_seen_at (attribution time) in UTC; caller should
// convert from MSK if needed.
//
// Everything is fetched with a single SQL query: the per-user dedup via
// DISTINCT ON, and the deposit aggregates via a grouped join on
// conversion_events (indexed by attribution_id). The previous version issued
// two extra queries per referred player (N+1), which made the page take
// seconds or minutes in production.
func (s *Service) GetB2CReferrals(ctx context.Context, partnerID string, from, to *time.Time) (total int, sum int64, items []B2CReferralItem, err error) {
	// Get partner revshare for income calc.
	profile, err := s.q(ctx).GetPartnerProfileByID(ctx, partnerID)
	if err != nil {
		return 0, 0, nil, err
	}
	commissionPercent := int(profile.RevsharePercentBps / 100) // 4000 bps -> 40%
	revshareBps := int(profile.RevsharePercentBps)

	// $1 partner_id, then optional $n range bounds shared by both branches.
	// earliest: first-touch attribution per referred player (dedup by
	// external_user_id) within the requested first_seen_at range.
	// deposits: all conversion events of the partner's players, grouped per
	// player, within the requested occurred_at range.
	query := `
		WITH earliest AS (
			SELECT DISTINCT ON (a.external_user_id)
			       a.id, a.external_user_id, a.first_seen_at,
			       tl.id AS link_id, tl.name AS link_name, tl.type AS link_type
			FROM external_user_attributions a
			LEFT JOIN tracking_clicks tc ON tc.id = a.tracking_click_id
			LEFT JOIN tracking_links tl ON tl.id = COALESCE(a.tracking_link_id, tc.tracking_link_id)
			WHERE a.partner_id = $1`
	args := []interface{}{partnerID}
	argIdx := 2
	if from != nil {
		query += fmt.Sprintf(" AND a.first_seen_at >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND a.first_seen_at <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}
	query += `
			ORDER BY a.external_user_id, a.first_seen_at
		), deposits AS (
			SELECT a.external_user_id,
			       count(*) AS deposits_count,
			       COALESCE(sum(ce.amount_kopecks), 0) AS deposits_sum
			FROM conversion_events ce
			JOIN external_user_attributions a ON a.id = ce.attribution_id
			WHERE a.partner_id = $1`
	if from != nil {
		query += fmt.Sprintf(" AND ce.occurred_at >= $%d", argIdx)
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND ce.occurred_at <= $%d", argIdx)
		args = append(args, *to)
		argIdx++
	}
	query += `
			GROUP BY a.external_user_id
		)
		SELECT e.external_user_id, e.first_seen_at, e.link_id, e.link_name, e.link_type,
		       COALESCE(d.deposits_count, 0), COALESCE(d.deposits_sum, 0)
		FROM earliest e
		LEFT JOIN deposits d ON d.external_user_id = e.external_user_id
		ORDER BY e.first_seen_at DESC`

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()

	items = make([]B2CReferralItem, 0, 64)
	for rows.Next() {
		var (
			externalUserID             string
			firstSeenAt                time.Time
			linkID, linkName, linkType *string
			depositsCount              int64
			depositsSum                int64
		)
		if err := rows.Scan(&externalUserID, &firstSeenAt, &linkID, &linkName, &linkType, &depositsCount, &depositsSum); err != nil {
			return 0, 0, nil, err
		}
		kind := "registration"
		if linkType != nil && *linkType == "promo" {
			kind = "promo"
		}
		sourceID, sourceName := "", ""
		if linkID != nil {
			sourceID = *linkID
		}
		if linkName != nil {
			sourceName = *linkName
		}
		it := B2CReferralItem{
			// Name fallback: resolve via an external system mapping later if needed.
			UserID:     externalUserID,
			Name:       externalUserID,
			Kind:       kind,
			CreatedAt:  firstSeenAt.UTC().Format(time.RFC3339),
			SourceID:   sourceID,
			SourceName: sourceName,
		}
		it.DepositsCount = depositsCount
		it.DepositsSum = depositsSum
		it.Income = depositsSum * int64(revshareBps) / 10000
		it.CommissionPercent = commissionPercent
		items = append(items, it)
		sum += it.Income
	}
	if err := rows.Err(); err != nil {
		return 0, 0, nil, err
	}

	// Rows are already ordered by first_seen_at DESC; search filtering and
	// pagination are applied by the caller (API layer).
	total = len(items)
	return total, sum, items, nil
}

// ParseRange parses the time range from query params (MSK).
func ParseRange(fromStr, toStr string) (from, to *time.Time) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	if fromStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", fromStr, loc); err == nil {
			tt := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			from = &tt
		} else if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		}
	}
	if toStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", toStr, loc); err == nil {
			tt := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc)
			to = &tt
		} else if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		}
	}
	return
}
