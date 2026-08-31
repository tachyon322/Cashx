package offers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"cashx/internal/platform"
	"cashx/internal/repository"
)

// normalizeDomainURL mirrors kazik's normalizeDomainUrl: ensures scheme, returns origin lowercased.
func normalizeDomainURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: invalid_domain", platform.ErrValidation)
	}
	if !strings.HasPrefix(strings.ToLower(s), "http://") && !strings.HasPrefix(strings.ToLower(s), "https://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("%w: invalid_domain", platform.ErrValidation)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: invalid_domain", platform.ErrValidation)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: invalid_domain", platform.ErrValidation)
	}
	// origin only, lowercased
	origin := strings.ToLower(u.Scheme + "://" + u.Host)
	// Validate via url.Parse again
	if _, err := url.Parse(origin); err != nil {
		return "", fmt.Errorf("%w: invalid_domain", platform.ErrValidation)
	}
	return origin, nil
}

// EnsureDefaultDomain creates a default domain entry if none exist.
// It uses WebOrigin as DEFAULT_ORIGIN (like kazik's FRONTEND_ORIGIN).
func (s *Service) EnsureDefaultDomain(ctx context.Context) error {
	q := s.q(ctx)
	rows, err := q.ListPartnerDomains(ctx)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	origin := s.WebOrigin
	if origin == "" {
		origin = "http://localhost:3000"
	}
	norm, err := normalizeDomainURL(origin)
	if err != nil {
		return err
	}
	_, err = q.CreatePartnerDomain(ctx, repository.CreatePartnerDomainParams{
		Url:                 norm,
		IsActive:            true,
		Comment:             repository.TextPtr(stringPtr("Основной домен приложения")),
		LegacyKazikDomainID: pgtype.Text{},
	})
	if err != nil && !isUniqueViolation(err) {
		return err
	}
	return nil
}

func stringPtr(s string) *string { return &s }

// AllowedDomains returns active domain urls.
func (s *Service) AllowedDomains(ctx context.Context) ([]string, error) {
	if err := s.EnsureDefaultDomain(ctx); err != nil {
		return nil, err
	}
	rows, err := s.q(ctx).ListActivePartnerDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Url)
	}
	return out, nil
}

// DefaultDomain returns the primary domain to use when a source has no explicit domain.
func (s *Service) DefaultDomain(ctx context.Context) (string, error) {
	if err := s.EnsureDefaultDomain(ctx); err != nil {
		return "", err
	}
	rows, err := s.q(ctx).ListActivePartnerDomains(ctx)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		base := s.WebOrigin
		if base == "" {
			base = "http://localhost:3000"
		}
		norm, _ := normalizeDomainURL(base)
		return norm, nil
	}
	// Prefer matching WebOrigin
	base := s.WebOrigin
	if base != "" {
		if norm, err := normalizeDomainURL(base); err == nil {
			for _, r := range rows {
				if r.Url == norm {
					return r.Url, nil
				}
			}
		}
	}
	return rows[0].Url, nil
}

// resolveSourceDomain validates and normalizes a per-source domain.
// For promo type it always returns nil (no domain).
func (s *Service) resolveSourceDomain(ctx context.Context, raw *string, typ string) (*string, error) {
	if typ == "promo" {
		return nil, nil
	}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	norm, err := normalizeDomainURL(*raw)
	if err != nil {
		return nil, err
	}
	allowed, err := s.AllowedDomains(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range allowed {
		if a == norm {
			return &norm, nil
		}
	}
	return nil, fmt.Errorf("%w: domain_not_allowed", platform.ErrValidation)
}

// ListDomains returns all domains (admin).
func (s *Service) ListDomains(ctx context.Context) ([]repository.PartnerDomain, error) {
	return s.q(ctx).ListPartnerDomains(ctx)
}

// CreateDomain creates a partner domain.
func (s *Service) CreateDomain(ctx context.Context, rawURL string, isActive *bool, comment *string) (repository.PartnerDomain, error) {
	norm, err := normalizeDomainURL(rawURL)
	if err != nil {
		return repository.PartnerDomain{}, err
	}
	active := true
	if isActive != nil {
		active = *isActive
	}
	row, err := s.q(ctx).CreatePartnerDomain(ctx, repository.CreatePartnerDomainParams{
		Url:      norm,
		IsActive: active,
		Comment:  repository.TextPtr(comment),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return repository.PartnerDomain{}, fmt.Errorf("%w: domain_taken", platform.ErrConflict)
		}
		return repository.PartnerDomain{}, err
	}
	return row, nil
}

// UpdateDomain patches a domain.
func (s *Service) UpdateDomain(ctx context.Context, id string, rawURL *string, isActive *bool, comment *string) (repository.PartnerDomain, error) {
	// Validate URL if provided
	var urlVal pgtype.Text
	if rawURL != nil {
		norm, err := normalizeDomainURL(*rawURL)
		if err != nil {
			return repository.PartnerDomain{}, err
		}
		urlVal = pgtype.Text{String: norm, Valid: true}
	}
	row, err := s.q(ctx).UpdatePartnerDomain(ctx, repository.UpdatePartnerDomainParams{
		ID:       id,
		Url:      urlVal,
		IsActive: repository.BoolPtr(isActive),
		Comment:  repository.TextPtr(comment),
	})
	if err != nil {
		return repository.PartnerDomain{}, err
	}
	return row, nil
}

// DeleteDomain removes a domain.
func (s *Service) DeleteDomain(ctx context.Context, id string) error {
	return s.q(ctx).DeletePartnerDomain(ctx, id)
}

// GetDomain returns a domain by ID.
func (s *Service) GetDomain(ctx context.Context, id string) (repository.PartnerDomain, error) {
	return s.q(ctx).GetPartnerDomainByID(ctx, id)
}
