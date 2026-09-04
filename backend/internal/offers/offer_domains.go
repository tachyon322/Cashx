package offers

import (
	"context"
	"fmt"
	"strings"

	"cashx/internal/platform"
	"cashx/internal/repository"
)

// OfferDomain is one domain (mirror) of an offer.
type OfferDomain struct {
	ID        string  `json:"id"`
	OfferID   string  `json:"offer_id"`
	URL       string  `json:"url"`
	IsMain    bool    `json:"is_main"`
	IsActive  bool    `json:"is_active"`
	Comment   *string `json:"comment"`
	CreatedAt string  `json:"created_at"`
}

func offerDomainFromRow(r repository.OfferDomain) OfferDomain {
	return OfferDomain{
		ID: r.ID, OfferID: r.OfferID, URL: r.Url,
		IsMain: r.IsMain, IsActive: r.IsActive,
		Comment:   repository.TextToPtr(r.Comment),
		CreatedAt: r.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// LinkURL builds the public tracking URL for a link code: the link's own
// domain, else the offer's active main domain, else the CashX tracker origin.
func (s *Service) LinkURL(ctx context.Context, offerID, domain, code string) string {
	mainDomain, _ := s.MainOfferDomain(ctx, offerID)
	return s.linkURL(code, domain, mainDomain)
}

// ListOfferDomains returns all domains of an offer (admin view).
func (s *Service) ListOfferDomains(ctx context.Context, offerID string) ([]OfferDomain, error) {
	rows, err := s.q(ctx).ListOfferDomains(ctx, offerID)
	if err != nil {
		return nil, err
	}
	out := make([]OfferDomain, 0, len(rows))
	for _, r := range rows {
		out = append(out, offerDomainFromRow(r))
	}
	return out, nil
}

// ListActiveOfferDomainsForPartner returns the active domains of an offer the
// partner is joined to (cabinet view).
func (s *Service) ListActiveOfferDomainsForPartner(ctx context.Context, partnerID, offerID string) ([]OfferDomain, error) {
	q := s.q(ctx)
	if _, err := s.accessForOffer(ctx, q, partnerID, offerID); err != nil {
		return nil, err
	}
	rows, err := q.ListActiveOfferDomains(ctx, offerID)
	if err != nil {
		return nil, err
	}
	out := make([]OfferDomain, 0, len(rows))
	for _, r := range rows {
		out = append(out, offerDomainFromRow(r))
	}
	return out, nil
}

// MainOfferDomain returns the active main domain of an offer, if any.
func (s *Service) MainOfferDomain(ctx context.Context, offerID string) (string, error) {
	row, err := s.q(ctx).GetMainOfferDomain(ctx, offerID)
	if err != nil {
		return "", nil // no main domain — not an error
	}
	return row.Url, nil
}

// CreateOfferDomain adds a domain to an offer. Exactly one main domain per
// offer is enforced by clearing the previous main flag.
func (s *Service) CreateOfferDomain(ctx context.Context, offerID, rawURL string, isMain bool, isActive *bool, comment *string) (OfferDomain, error) {
	norm, err := normalizeDomainURL(rawURL)
	if err != nil {
		return OfferDomain{}, err
	}
	active := true
	if isActive != nil {
		active = *isActive
	}
	var row repository.OfferDomain
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		if isMain && active {
			if err := tq.ClearMainOfferDomain(ctx, offerID); err != nil {
				return err
			}
		}
		var e error
		row, e = tq.CreateOfferDomain(ctx, repository.CreateOfferDomainParams{
			OfferID: offerID, Url: norm, IsMain: isMain, IsActive: active,
			Comment: repository.TextPtr(comment),
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return OfferDomain{}, fmt.Errorf("%w: domain_taken", platform.ErrConflict)
		}
		return OfferDomain{}, err
	}
	return offerDomainFromRow(row), nil
}

// UpdateOfferDomain patches a domain (url/activity/comment) and optionally
// promotes it to main (clearing the previous main).
func (s *Service) UpdateOfferDomain(ctx context.Context, offerID, id string, rawURL *string, isActive *bool, isMain *bool, comment *string) (OfferDomain, error) {
	q := s.q(ctx)
	if _, err := q.GetOfferDomainByOfferAndID(ctx, repository.GetOfferDomainByOfferAndIDParams{
		ID: id, OfferID: offerID,
	}); err != nil {
		return OfferDomain{}, fmt.Errorf("%w: domain_not_found", platform.ErrNotFound)
	}
	var urlVal *string
	if rawURL != nil && strings.TrimSpace(*rawURL) != "" {
		norm, err := normalizeDomainURL(*rawURL)
		if err != nil {
			return OfferDomain{}, err
		}
		urlVal = &norm
	}
	var row repository.OfferDomain
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		var e error
		row, e = tq.UpdateOfferDomain(ctx, repository.UpdateOfferDomainParams{
			ID:       id,
			Url:      repository.TextPtr(urlVal),
			IsActive: repository.BoolPtr(isActive),
			Comment:  repository.TextPtr(comment),
		})
		if e != nil {
			return e
		}
		if isMain != nil {
			if *isMain {
				if e := tq.ClearMainOfferDomain(ctx, offerID); e != nil {
					return e
				}
			}
			if e := tq.SetOfferDomainMain(ctx, repository.SetOfferDomainMainParams{
				ID: id, IsMain: *isMain,
			}); e != nil {
				return e
			}
			row, e = tq.GetOfferDomainByID(ctx, id)
		}
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return OfferDomain{}, fmt.Errorf("%w: domain_taken", platform.ErrConflict)
		}
		return OfferDomain{}, err
	}
	return offerDomainFromRow(row), nil
}

// DeleteOfferDomain removes a domain from an offer.
func (s *Service) DeleteOfferDomain(ctx context.Context, offerID, id string) error {
	q := s.q(ctx)
	if _, err := q.GetOfferDomainByOfferAndID(ctx, repository.GetOfferDomainByOfferAndIDParams{
		ID: id, OfferID: offerID,
	}); err != nil {
		return fmt.Errorf("%w: domain_not_found", platform.ErrNotFound)
	}
	return q.DeleteOfferDomain(ctx, id)
}

// ValidateSourceDomain validates a link-source domain against the offer's
// active domains (used by the PATCH path that persists the field directly).
func (s *Service) ValidateSourceDomain(ctx context.Context, offerID, raw string) (*string, error) {
	return s.resolveSourceDomainForOffer(ctx, offerID, &raw, "link")
}

// resolveSourceDomainForOffer validates and normalizes a per-source domain
// against the active domains of the offer. Promo sources never carry a domain.
func (s *Service) resolveSourceDomainForOffer(ctx context.Context, offerID string, raw *string, typ string) (*string, error) {
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
	rows, err := s.q(ctx).ListActiveOfferDomains(ctx, offerID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Url == norm {
			return &norm, nil
		}
	}
	return nil, fmt.Errorf("%w: domain_not_allowed", platform.ErrValidation)
}
