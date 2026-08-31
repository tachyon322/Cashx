package offers

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"cashx/internal/platform"
	"cashx/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
)

// normalizeRedirectURL mirrors kazik's normalizeRedirectUrl.
func normalizeRedirectURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: invalid_url", platform.ErrValidation)
	}
	if !strings.HasPrefix(strings.ToLower(s), "http://") && !strings.HasPrefix(strings.ToLower(s), "https://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("%w: invalid_url", platform.ErrValidation)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: invalid_url", platform.ErrValidation)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: invalid_url", platform.ErrValidation)
	}
	return u.String(), nil
}

// WeightedPick selects a URL weighted random (kazik service.ts:194).
func WeightedPick(urls []repository.RedirectPoolUrl) string {
	active := make([]repository.RedirectPoolUrl, 0, len(urls))
	for _, u := range urls {
		if u.IsActive && strings.TrimSpace(u.Url) != "" {
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
			return strings.TrimSpace(u.Url)
		}
	}
	return strings.TrimSpace(active[0].Url)
}

// ListRedirectPools returns all pools with their URLs.
func (s *Service) ListRedirectPools(ctx context.Context) ([]repository.RedirectPool, error) {
	return s.q(ctx).ListRedirectPools(ctx)
}

// ListRedirectPoolsWithURLs returns pools with nested URLs (for admin).
type RedirectPoolWithURLs struct {
	Pool repository.RedirectPool     `json:"pool"`
	URLs []repository.RedirectPoolUrl `json:"urls"`
}

func (s *Service) ListRedirectPoolsWithURLs(ctx context.Context) ([]RedirectPoolWithURLs, error) {
	pools, err := s.q(ctx).ListRedirectPools(ctx)
	if err != nil {
		return nil, err
	}
	if len(pools) == 0 {
		return []RedirectPoolWithURLs{}, nil
	}
	out := make([]RedirectPoolWithURLs, 0, len(pools))
	for _, p := range pools {
		urls, err := s.q(ctx).ListRedirectPoolURLs(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, RedirectPoolWithURLs{Pool: p, URLs: urls})
	}
	return out, nil
}

// CreateRedirectPool creates a pool and optionally URLs.
func (s *Service) CreateRedirectPool(ctx context.Context, name string, comment *string, urls []string) (repository.RedirectPool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return repository.RedirectPool{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	q := s.q(ctx)
	pool, err := q.CreateRedirectPool(ctx, repository.CreateRedirectPoolParams{
		Name:                 name,
		Comment:              repository.TextPtr(comment),
		LegacyKazikRedirectID: pgtype.Text{},
	})
	if err != nil {
		return repository.RedirectPool{}, err
	}
	if len(urls) > 0 {
		for i, raw := range urls {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			norm, err := normalizeRedirectURL(raw)
			if err != nil {
				return repository.RedirectPool{}, err
			}
			_, err = q.CreateRedirectPoolURL(ctx, repository.CreateRedirectPoolURLParams{
				RedirectID: pool.ID,
				Url:        norm,
				Weight:     1,
				IsActive:   true,
				SortOrder:  int32(i),
			})
			if err != nil {
				return repository.RedirectPool{}, err
			}
		}
	}
	return pool, nil
}

// UpdateRedirectPool patches name/comment.
func (s *Service) UpdateRedirectPool(ctx context.Context, id string, name *string, comment *string) (repository.RedirectPool, error) {
	var namePtr *string
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return repository.RedirectPool{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
		}
		namePtr = &n
	}
	row, err := s.q(ctx).UpdateRedirectPool(ctx, repository.UpdateRedirectPoolParams{
		ID:      id,
		Name:    repository.TextPtr(namePtr),
		Comment: repository.TextPtr(comment),
	})
	if err != nil {
		return repository.RedirectPool{}, err
	}
	return row, nil
}

// DeleteRedirectPool removes a pool (cascade URLs).
func (s *Service) DeleteRedirectPool(ctx context.Context, id string) error {
	return s.q(ctx).DeleteRedirectPool(ctx, id)
}

// AddRedirectPoolURL adds a URL to a pool.
func (s *Service) AddRedirectPoolURL(ctx context.Context, redirectID, rawURL string, weight *int) (repository.RedirectPoolUrl, error) {
	norm, err := normalizeRedirectURL(rawURL)
	if err != nil {
		return repository.RedirectPoolUrl{}, err
	}
	w := 1
	if weight != nil {
		w = *weight
		if w < 1 {
			w = 1
		}
	}
	// next sort_order = count
	cnt, err := s.q(ctx).CountRedirectPoolURLs(ctx, redirectID)
	if err != nil {
		return repository.RedirectPoolUrl{}, err
	}
	row, err := s.q(ctx).CreateRedirectPoolURL(ctx, repository.CreateRedirectPoolURLParams{
		RedirectID: redirectID,
		Url:        norm,
		Weight:     int32(w),
		IsActive:   true,
		SortOrder:  int32(cnt),
	})
	return row, err
}

// UpdateRedirectPoolURL patches url/weight/isActive.
func (s *Service) UpdateRedirectPoolURL(ctx context.Context, redirectID, urlID string, rawURL *string, weight *int, isActive *bool) (repository.RedirectPoolUrl, error) {
	var urlVal pgtype.Text
	if rawURL != nil {
		norm, err := normalizeRedirectURL(*rawURL)
		if err != nil {
			return repository.RedirectPoolUrl{}, err
		}
		urlVal = pgtype.Text{String: norm, Valid: true}
	}
	var weightVal pgtype.Int4
	if weight != nil {
		w := *weight
		if w < 1 {
			w = 1
		}
		weightVal = pgtype.Int4{Int32: int32(w), Valid: true}
	}
	var activeVal pgtype.Bool
	if isActive != nil {
		activeVal = pgtype.Bool{Bool: *isActive, Valid: true}
	}
	row, err := s.q(ctx).UpdateRedirectPoolURL(ctx, repository.UpdateRedirectPoolURLParams{
		ID:         urlID,
		RedirectID: redirectID,
		Url:        urlVal,
		Weight:     weightVal,
		IsActive:   activeVal,
	})
	return row, err
}

// DeleteRedirectPoolURL removes a URL.
func (s *Service) DeleteRedirectPoolURL(ctx context.Context, redirectID, urlID string) error {
	return s.q(ctx).DeleteRedirectPoolURL(ctx, repository.DeleteRedirectPoolURLParams{
		ID:         urlID,
		RedirectID: redirectID,
	})
}

// ListRedirectPoolURLs returns URLs for a pool.
func (s *Service) ListRedirectPoolURLs(ctx context.Context, redirectID string) ([]repository.RedirectPoolUrl, error) {
	return s.q(ctx).ListRedirectPoolURLs(ctx, redirectID)
}
