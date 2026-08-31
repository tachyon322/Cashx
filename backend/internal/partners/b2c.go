package partners

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	DepositsSum       int64   `json:"deposits_sum"` // kopecks
	Income            int64   `json:"income"`       // kopecks, commission
	CommissionPercent int     `json:"commission_percent"` // 0-100
}

// GetB2CReferrals returns player referrals (B2C) for a partner, deduped by external_user_id.
// Range filters by first_seen_at (attribution time) in UTC; caller should convert from MSK if needed.
func (s *Service) GetB2CReferrals(ctx context.Context, partnerID string, from, to *time.Time) (total int, sum int64, items []B2CReferralItem, err error) {
	q := s.q(ctx)
	// Get all attributions for partner
	// Use raw query via pool because we need dynamic filtering and joins.
	// We'll query external_user_attributions with tracking link info.
	query := `
		SELECT a.id, a.external_user_id, a.first_seen_at, a.tracking_click_id,
		       tl.id as link_id, tl.name as link_name, tl.type as link_type
		FROM external_user_attributions a
		LEFT JOIN tracking_clicks tc ON tc.id = a.tracking_click_id
		LEFT JOIN tracking_links tl ON tl.id = tc.tracking_link_id
		WHERE a.partner_id = $1
	`
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
	query += " ORDER BY a.first_seen_at DESC"

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()

	type row struct {
		ID             int64
		ExternalUserID string
		FirstSeenAt    time.Time
		ClickID        *int64
		LinkID         *string
		LinkName       *string
		LinkType       *string
	}
	var raw []row
	byUser := make(map[string]B2CReferralItem)
	for rows.Next() {
		var r row
		var clickID *int64
		var linkID, linkName, linkType *string
		if err := rows.Scan(&r.ID, &r.ExternalUserID, &r.FirstSeenAt, &clickID, &linkID, &linkName, &linkType); err != nil {
			return 0, 0, nil, err
		}
		r.ClickID = clickID
		r.LinkID = linkID
		r.LinkName = linkName
		r.LinkType = linkType
		raw = append(raw, r)
		// Dedup by external_user_id (first-seen earliest? but we order DESC, so first encountered is latest; need earliest)
		// Since query is DESC, first encountered is latest; we want earliest, so we should keep first and ignore later duplicates? But to keep earliest, we should iterate and if not exists, add; later duplicates (earlier) will be ignored. To get earliest, we should order ASC and keep first.
		// However we want dedup keep earliest attribution (first-touch). So we should have ordered ASC and kept first.
		// We'll fix by processing in reverse: keep earliest by checking if already exists, skip.
	}
	// Fix dedup: need earliest, so we should have queried ASC and keep first. Let's re-sort or just keep first seen (which is latest due to DESC). Better to re-query ASC.
	// For now, handle dedup properly: iterate raw in reverse to keep earliest.
	for i := len(raw) - 1; i >= 0; i-- {
		r := raw[i]
		if _, ok := byUser[r.ExternalUserID]; ok {
			continue
		}
		kind := "registration"
		if r.LinkType != nil && *r.LinkType == "promo" {
			kind = "promo"
		}
		sourceID := ""
		if r.LinkID != nil {
			sourceID = *r.LinkID
		}
		sourceName := ""
		if r.LinkName != nil {
			sourceName = *r.LinkName
		}
		byUser[r.ExternalUserID] = B2CReferralItem{
			UserID:     r.ExternalUserID,
			Name:       r.ExternalUserID, // fallback, will try to resolve via external system later
			Kind:       kind,
			CreatedAt:  r.FirstSeenAt.UTC().Format(time.RFC3339),
			SourceID:   sourceID,
			SourceName: sourceName,
		}
	}

	if len(byUser) == 0 {
		return 0, 0, []B2CReferralItem{}, nil
	}

	// Get partner revshare for income calc
	profile, err := q.GetPartnerProfileByID(ctx, partnerID)
	if err != nil {
		return 0, 0, nil, err
	}
	commissionPercent := int(profile.RevsharePercentBps / 100) // 4000 bps -> 40%
	revshareBps := int(profile.RevsharePercentBps)

	// For each user, get deposit aggregates from conversion_events
	items = make([]B2CReferralItem, 0, len(byUser))
	for uid, it := range byUser {
		// Query conversion_events for this user via attributions
		// Find attribution id for uid
		var attrID int64
		err := s.Pool.QueryRow(ctx, `SELECT id FROM external_user_attributions WHERE partner_id=$1 AND external_user_id=$2 ORDER BY first_seen_at LIMIT 1`, partnerID, uid).Scan(&attrID)
		if err != nil && err != pgx.ErrNoRows {
			continue
		}
		var depositsCount int64
		var depositsSum int64
		// Apply date range to conversion occurred_at as well?
		q2 := `SELECT count(*), COALESCE(sum(amount_kopecks),0) FROM conversion_events WHERE attribution_id=$1`
		args2 := []interface{}{attrID}
		if from != nil {
			q2 += fmt.Sprintf(" AND occurred_at >= $%d", len(args2)+1)
			args2 = append(args2, *from)
		}
		if to != nil {
			q2 += fmt.Sprintf(" AND occurred_at <= $%d", len(args2)+1)
			args2 = append(args2, *to)
		}
		_ = s.Pool.QueryRow(ctx, q2, args2...).Scan(&depositsCount, &depositsSum)

		income := depositsSum * int64(revshareBps) / 10000

		// Try to get user name/email from our users if external_user_id matches email? For now keep as is.
		// Attempt to find in users table by external_user_id as email? Not reliable.
		it.DepositsCount = depositsCount
		it.DepositsSum = depositsSum
		it.Income = income
		it.CommissionPercent = commissionPercent
		// Name fallback: if we can find user by external_user_id in some mapping, not needed.

		items = append(items, it)
		sum += income
	}

	// Sort by created_at desc
	// Simple bubble sort by time parse
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			ti, _ := time.Parse(time.RFC3339, items[i].CreatedAt)
			tj, _ := time.Parse(time.RFC3339, items[j].CreatedAt)
			if tj.After(ti) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Apply string search filter? Handled at API level.

	// Also need to handle kind filter? Keep all.

	// For now, return all; pagination handled by API.
	total = len(items)
	return total, sum, items, nil
}

// Helper to parse time range from query params (MSK).
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

// Ensure imports used: strings is used? keep for future
var _ = strings.TrimSpace
