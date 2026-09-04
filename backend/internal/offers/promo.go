package offers

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"cashx/internal/platform"
	"cashx/internal/repository"
)

// PromoFallbackBonus is used when a promo has no explicit registration_bonus (like kazik PROMO_FALLBACK_BONUS=500).
const PromoFallbackBonus = 500

// ResolvePromoCode finds a promo tracking link by code.
func (s *Service) ResolvePromoCode(ctx context.Context, rawCode string) (*repository.TrackingLink, int, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil, 0, fmt.Errorf("%w: invalid_code", platform.ErrValidation)
	}
	row, err := s.q(ctx).ResolvePromoCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, 0, fmt.Errorf("%w: promo_not_found", platform.ErrNotFound)
		}
		return nil, 0, err
	}
	amount := PromoFallbackBonus
	if row.RegistrationBonus.Valid {
		amount = int(row.RegistrationBonus.Int32)
	}
	return &row, amount, nil
}

// GetRegistrationBonus returns the bonus for a ref code (promo or link).
// If ref is a promo code, returns its registration_bonus, else returns default welcome bonus (0 for now, can be extended via platform_settings).
func (s *Service) GetRegistrationBonus(ctx context.Context, ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, nil
	}
	// Try promo first
	if row, amount, err := s.ResolvePromoCode(ctx, ref); err == nil && row != nil {
		return amount, nil
	}
	// Try link with registration_bonus override
	upper := strings.ToUpper(ref)
	_ = upper
	// Check if link has bonus
	row, err := s.q(ctx).GetTrackingLinkByCodeExtended(ctx, strings.ToUpper(ref))
	if err == nil && row.RegistrationBonus.Valid {
		return int(row.RegistrationBonus.Int32), nil
	}
	// Default welcome bonus could be from platform_settings or env; keep 0 for now
	return 0, nil
}

// CreatePromoSource creates a promo-type tracking link.
func (s *Service) CreatePromoSource(ctx context.Context, partnerID, offerID, name string, code *string, registrationBonus *int, comment, groupID *string) (Source, error) {
	typ := "promo"
	// promo never has domain/redirect
	return s.createSourceWithType(ctx, partnerID, offerID, name, code, comment, groupID, typ, registrationBonus, nil, nil, false)
}

// CreateLinkSource creates a link-type tracking link with optional domain/redirect.
func (s *Service) CreateLinkSource(ctx context.Context, partnerID, offerID, name string, code *string, comment, groupID *string, domain *string, redirectID *string, isDefault bool) (Source, error) {
	return s.createSourceWithType(ctx, partnerID, offerID, name, code, comment, groupID, "link", nil, domain, redirectID, isDefault)
}

// createSourceWithType is the internal helper handling both link and promo.
func (s *Service) createSourceWithType(ctx context.Context, partnerID, offerID, name string, code *string, comment, groupID *string, typ string, registrationBonus *int, domain *string, redirectID *string, isDefault bool) (Source, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Source{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if typ != "link" && typ != "promo" {
		return Source{}, fmt.Errorf("%w: invalid_type", platform.ErrValidation)
	}
	if typ == "promo" && registrationBonus != nil && *registrationBonus < 0 {
		return Source{}, fmt.Errorf("%w: invalid_bonus", platform.ErrValidation)
	}
	q := s.q(ctx)
	access, err := s.accessForOffer(ctx, q, partnerID, offerID)
	if err != nil {
		return Source{}, err
	}
	normalized := ""
	if code != nil && strings.TrimSpace(*code) != "" {
		if normalized, err = validateCode(*code); err != nil {
			return Source{}, err
		}
	} else {
		if normalized, err = genLinkCode(ctx, q); err != nil {
			return Source{}, err
		}
	}
	// Domain validation
	var domainVal *string
	if typ != "promo" && domain != nil {
		if domainVal, err = s.resolveSourceDomainForOffer(ctx, offerID, domain, typ); err != nil {
			return Source{}, err
		}
	}
	// Redirect validation
	var redirectPtr *string
	if redirectID != nil && strings.TrimSpace(*redirectID) != "" {
		if typ == "promo" {
			// promo ignores redirect
			redirectPtr = nil
		} else {
			// validate exists
			if _, err := q.GetRedirectPoolByID(ctx, *redirectID); err != nil {
				if err == pgx.ErrNoRows {
					return Source{}, fmt.Errorf("%w: redirect_not_found", platform.ErrNotFound)
				}
				return Source{}, err
			}
			redirectPtr = redirectID
		}
	}
	groupPg := repository.UUIDPtr(groupID)
	if groupID != nil && *groupID != "" {
		if _, err := s.ownedGroup(ctx, q, partnerID, *groupID); err != nil {
			return Source{}, err
		}
	}
	// Prepare registration_bonus pgtype
	var bonusVal *int32
	if registrationBonus != nil {
		b := int32(*registrationBonus)
		bonusVal = &b
	}
	// Need to handle domain and redirect as pgtype for insertion via full query
	// Use CreateTrackingLinkFull for extended fields
	var link repository.TrackingLink
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		// Use full insert
		var err error
		link, err = tq.CreateTrackingLinkFull(ctx, repository.CreateTrackingLinkFullParams{
			PartnerOfferAccessID: access.ID,
			Code:                 normalized,
			Name:                 name,
			Comment:              repository.TextPtr(comment),
			GroupID:              groupPg,
			IsDefault:            isDefault,
			IsActive:             true,
			Type:                 repository.TextPtr(&typ),
			RegistrationBonus:    repository.Int32Ptr(bonusValToIntPtr(bonusVal)),
			Domain:               repository.TextPtr(domainVal),
			RedirectID:           repository.UUIDPtr(redirectPtr),
			LegacyKazikSourceID:  repository.TextPtr(nil),
		})
		if err != nil {
			return err
		}
		if isDefault {
			if err := tq.ClearDefaultTrackingLinks(ctx, access.ID); err != nil {
				return err
			}
			return tq.SetDefaultTrackingLink(ctx, link.ID)
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Source{}, fmt.Errorf("%w: code_taken", platform.ErrConflict)
		}
		return Source{}, err
	}
	return s.sourceByID(ctx, q, offerID, link.ID)
}

func bonusValToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}
