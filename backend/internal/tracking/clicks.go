package tracking

import (
	"context"
	"errors"
	"fmt"

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
	click, err := q.CreateTrackingClick(ctx, repository.CreateTrackingClickParams{
		TrackingLinkID: link.ID,
		Column2:        ip,
		UserAgent:      repository.TextPtr(&userAgent),
		Referrer:       repository.TextPtr(&referrer),
	})
	if err != nil {
		return ClickResult{}, err
	}
	dest := ""
	if link.OfferDestinationUrl.Valid {
		dest = link.OfferDestinationUrl.String
	}
	if dest == "" {
		dest = link.ProjectDestinationUrl
	}
	return ClickResult{ClickID: click.ID, Destination: dest}, nil
}
