package partners

import (
	"context"
	"sort"
	"strconv"
	"time"
)

// ActivityItem is one entry of the partner-wide recent activity feed.
// It mirrors kazik's AffiliateHistoryItem: mixed clicks / registrations /
// payments / commission earnings, newest first.
type ActivityItem struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"` // click|registration|payment|earning|reversal
	SourceName    string `json:"source_name"`
	OfferName     string `json:"offer_name"`
	AmountKopecks *int64 `json:"amount_kopecks,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

// GetRecentActivity returns the merged activity feed for a partner:
// tracking clicks, attributions (registrations), conversion events
// (payments) and commission earnings, newest first, sliced to limit.
// Each source table is pre-limited so the merge stays cheap.
func (s *Service) GetRecentActivity(ctx context.Context, partnerID string, limit int) ([]ActivityItem, error) {
	return s.getRecentActivity(ctx, partnerID, "", limit)
}

// GetOfferActivity is GetRecentActivity scoped to one offer (offer page feed).
func (s *Service) GetOfferActivity(ctx context.Context, partnerID, offerID string, limit int) ([]ActivityItem, error) {
	return s.getRecentActivity(ctx, partnerID, offerID, limit)
}

func (s *Service) getRecentActivity(ctx context.Context, partnerID, offerID string, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	perSource := limit
	if perSource < 50 {
		perSource = 50
	}

	type row struct {
		item ActivityItem
		at   time.Time
	}
	var out []row

	// Optional offer scope, shared by all four source queries.
	clickOffer := ""
	attrOffer := ""
	earnOffer := ""
	args := []any{partnerID, perSource}
	if offerID != "" {
		clickOffer = " AND a.offer_id = $3"
		attrOffer = " AND a.offer_id = $3"
		earnOffer = " AND e.offer_id = $3"
		args = append(args, offerID)
	}

	// 1. Clicks through the partner's tracking links.
	clickRows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.created_at,
		       COALESCE(NULLIF(tl.name, ''), tl.code) AS source_name,
		       o.name AS offer_name
		FROM tracking_clicks c
		JOIN tracking_links tl ON tl.id = c.tracking_link_id
		JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
		JOIN offers o ON o.id = a.offer_id
		WHERE a.partner_id = $1`+clickOffer+`
		ORDER BY c.created_at DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, err
	}
	for clickRows.Next() {
		var id int64
		var at time.Time
		var source, offer string
		if err := clickRows.Scan(&id, &at, &source, &offer); err != nil {
			clickRows.Close()
			return nil, err
		}
		out = append(out, row{item: ActivityItem{
			ID:         "click_" + strconv.FormatInt(id, 10),
			Kind:       "click",
			SourceName: source,
			OfferName:  offer,
			OccurredAt: at.UTC().Format(time.RFC3339),
		}, at: at})
	}
	clickRows.Close()
	if err := clickRows.Err(); err != nil {
		return nil, err
	}

	// 2. Registrations (attributions), with link-name fallback via the click.
	attrRows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.first_seen_at,
		       COALESCE(NULLIF(tl1.name, ''), tl1.code, NULLIF(tl2.name, ''), tl2.code, o.name) AS source_name,
		       o.name AS offer_name
		FROM external_user_attributions a
		JOIN offers o ON o.id = a.offer_id
		LEFT JOIN tracking_links tl1 ON tl1.id = a.tracking_link_id
		LEFT JOIN tracking_clicks tc ON tc.id = a.tracking_click_id
		LEFT JOIN tracking_links tl2 ON tl2.id = tc.tracking_link_id
		WHERE a.partner_id = $1`+attrOffer+`
		ORDER BY a.first_seen_at DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, err
	}
	for attrRows.Next() {
		var id int64
		var at time.Time
		var source, offer string
		if err := attrRows.Scan(&id, &at, &source, &offer); err != nil {
			attrRows.Close()
			return nil, err
		}
		out = append(out, row{item: ActivityItem{
			ID:         "reg_" + strconv.FormatInt(id, 10),
			Kind:       "registration",
			SourceName: source,
			OfferName:  offer,
			OccurredAt: at.UTC().Format(time.RFC3339),
		}, at: at})
	}
	attrRows.Close()
	if err := attrRows.Err(); err != nil {
		return nil, err
	}

	// 3. Payments (conversion events) of attributed users.
	convRows, err := s.Pool.Query(ctx, `
		SELECT ce.id, ce.amount_kopecks, ce.occurred_at,
		       COALESCE(NULLIF(tl1.name, ''), tl1.code, NULLIF(tl2.name, ''), tl2.code, o.name) AS source_name,
		       o.name AS offer_name
		FROM conversion_events ce
		JOIN external_user_attributions a ON a.id = ce.attribution_id
		JOIN offers o ON o.id = a.offer_id
		LEFT JOIN tracking_links tl1 ON tl1.id = a.tracking_link_id
		LEFT JOIN tracking_clicks tc ON tc.id = a.tracking_click_id
		LEFT JOIN tracking_links tl2 ON tl2.id = tc.tracking_link_id
		WHERE a.partner_id = $1`+attrOffer+`
		ORDER BY ce.occurred_at DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, err
	}
	for convRows.Next() {
		var id int64
		var amount int64
		var at time.Time
		var source, offer string
		if err := convRows.Scan(&id, &amount, &at, &source, &offer); err != nil {
			convRows.Close()
			return nil, err
		}
		amt := amount
		out = append(out, row{item: ActivityItem{
			ID:            "pay_" + strconv.FormatInt(id, 10),
			Kind:          "payment",
			SourceName:    source,
			OfferName:     offer,
			AmountKopecks: &amt,
			OccurredAt:    at.UTC().Format(time.RFC3339),
		}, at: at})
	}
	convRows.Close()
	if err := convRows.Err(); err != nil {
		return nil, err
	}

	// 4. Commission earnings (reversed_at set => reversal / сторно).
	earnRows, err := s.Pool.Query(ctx, `
		SELECT e.id, e.amount_kopecks, e.reversed_at, e.created_at,
		       COALESCE(NULLIF(tl.name, ''), tl.code, o.name) AS source_name,
		       o.name AS offer_name
		FROM commission_earnings e
		JOIN offers o ON o.id = e.offer_id
		LEFT JOIN tracking_links tl ON tl.id = e.tracking_link_id
		WHERE e.partner_id = $1`+earnOffer+`
		ORDER BY e.created_at DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, err
	}
	for earnRows.Next() {
		var id string
		var amount int64
		var reversedAt *time.Time
		var at time.Time
		var source, offer string
		if err := earnRows.Scan(&id, &amount, &reversedAt, &at, &source, &offer); err != nil {
			earnRows.Close()
			return nil, err
		}
		kind := "earning"
		amt := amount
		if reversedAt != nil {
			kind = "reversal"
			amt = -amt
		}
		out = append(out, row{item: ActivityItem{
			ID:            "earn_" + id,
			Kind:          kind,
			SourceName:    source,
			OfferName:     offer,
			AmountKopecks: &amt,
			OccurredAt:    at.UTC().Format(time.RFC3339),
		}, at: at})
	}
	earnRows.Close()
	if err := earnRows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].at.After(out[j].at) })
	if len(out) > limit {
		out = out[:limit]
	}
	items := make([]ActivityItem, 0, len(out))
	for _, r := range out {
		items = append(items, r.item)
	}
	return items, nil
}
