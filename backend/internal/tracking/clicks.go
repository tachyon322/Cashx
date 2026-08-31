package tracking

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5"

	"cashx/internal/platform"
	"cashx/internal/repository"
)

// ClickResult is the outcome of recording a click.
type ClickResult struct {
	ClickID     int64
	Destination string
}

// RecordClick resolves a tracking link code, records the click and returns the
// destination URL for the redirect.
func RecordClick(ctx context.Context, q *repository.Queries, code, ip, userAgent, referrer string) (ClickResult, error) {
	link, err := q.GetTrackingLinkByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
		}
		return ClickResult{}, err
	}
	if !link.IsActive || link.AccessStatus != "active" {
		return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
	}
	// Promo codes are not redirectable via /c/
	if link.Type == "promo" {
		return ClickResult{}, fmt.Errorf("%w: link_not_found", platform.ErrNotFound)
	}
	click, err := q.CreateTrackingClick(ctx, repository.CreateTrackingClickParams{
		TrackingLinkID: link.ID,
		Column2:        ip,
		UserAgent:      repository.TextPtr(&userAgent),
		Referrer:       repository.TextPtr(&referrer),
	})
	if err != nil {
		return ClickResult{}, err
	}
	// Counters (best-effort)
	if DefaultCounters != nil {
		_ = DefaultCounters.RecordClick(ctx, link.ID, ClickMeta{IP: ip, UserAgent: userAgent, Referrer: referrer})
	}
	dest := ""
	// Weighted redirect pool takes precedence
	if link.RedirectID.Valid {
		urls, err := q.ListRedirectPoolURLs(ctx, link.RedirectID.String())
		if err == nil && len(urls) > 0 {
			dest = weightedPick(urls)
		}
	}
	if dest == "" && link.OfferDestinationUrl.Valid {
		dest = link.OfferDestinationUrl.String
	}
	if dest == "" {
		dest = link.ProjectDestinationUrl
	}
	return ClickResult{ClickID: click.ID, Destination: dest}, nil
}

func weightedPick(urls []repository.RedirectPoolUrl) string {
	active := make([]repository.RedirectPoolUrl, 0, len(urls))
	for _, u := range urls {
		if u.IsActive && u.Url != "" {
			active = append(active, u)
		}
	}
	if len(active) == 0 {
		return ""
	}
	total := 0
	for _, u := range active {
		w := int(u.Weight)
		if w < 1 {
			w = 1
		}
		total += w
	}
	r := rand.Intn(total)
	for _, u := range active {
		w := int(u.Weight)
		if w < 1 {
			w = 1
		}
		r -= w
		if r < 0 {
			return u.Url
		}
	}
	return active[0].Url
}
