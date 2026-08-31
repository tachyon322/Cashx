package offers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

var codePattern = regexp.MustCompile(`^[A-Z0-9]{1,32}$`)

// SourceTotals is aggregated per-source statistics.
type SourceTotals struct {
	Clicks        int64 `json:"clicks"`
	UniqueClicks  int64 `json:"unique_clicks"`
	Registrations int64 `json:"registrations"`
	FirstPayments int64 `json:"first_payments"`
	IncomeKopecks int64 `json:"income_kopecks"`
}

// Source is one tracking link (traffic source) with its statistics.
type Source struct {
	ID                string       `json:"id"`
	Code              string       `json:"code"`
	Name              string       `json:"name"`
	Comment           *string      `json:"comment"`
	GroupID           *string      `json:"group_id"`
	GroupName         *string      `json:"group_name"`
	IsDefault         bool         `json:"is_default"`
	IsActive          bool         `json:"is_active"`
	URL               string       `json:"url"`
	Type              string       `json:"type"`
	RegistrationBonus *int         `json:"registration_bonus,omitempty"`
	Domain            *string      `json:"domain,omitempty"`
	RedirectID        *string      `json:"redirect_id,omitempty"`
	Totals            SourceTotals `json:"totals"`
	Totals30d         SourceTotals `json:"totals_30d"`
	CreatedAt         string       `json:"created_at"`
}

// Group is a partner-scoped traffic flow used to organize sources.
type Group struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Comment   *string `json:"comment"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// accessForOffer loads the partner's active access to an offer.
func (s *Service) accessForOffer(ctx context.Context, q *repository.Queries, partnerID, offerID string) (repository.PartnerOfferAccess, error) {
	access, err := q.GetPartnerAccess(ctx, repository.GetPartnerAccessParams{PartnerID: partnerID, OfferID: offerID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return access, fmt.Errorf("%w: offer_not_joined", platform.ErrNotFound)
		}
		return access, err
	}
	if access.Status != "active" {
		return access, fmt.Errorf("%w: offer_not_joined", platform.ErrNotFound)
	}
	return access, nil
}

// ownedLink loads a tracking link and verifies it belongs to the partner's
// access for the given offer.
func (s *Service) ownedLink(ctx context.Context, q *repository.Queries, partnerID, offerID, sourceID string) (repository.TrackingLink, repository.PartnerOfferAccess, error) {
	var link repository.TrackingLink
	access, err := s.accessForOffer(ctx, q, partnerID, offerID)
	if err != nil {
		return link, access, err
	}
	link, err = q.GetTrackingLinkByID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return link, access, fmt.Errorf("%w: source_not_found", platform.ErrNotFound)
		}
		return link, access, err
	}
	if link.PartnerOfferAccessID != access.ID {
		return link, access, fmt.Errorf("%w: source_not_found", platform.ErrNotFound)
	}
	return link, access, nil
}

// ownedGroup loads a group and verifies it belongs to the partner.
func (s *Service) ownedGroup(ctx context.Context, q *repository.Queries, partnerID, groupID string) (repository.SourceGroup, error) {
	group, err := q.GetSourceGroupByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return group, fmt.Errorf("%w: group_not_found", platform.ErrNotFound)
		}
		return group, err
	}
	if group.PartnerID != partnerID {
		return group, fmt.Errorf("%w: group_not_found", platform.ErrNotFound)
	}
	return group, nil
}

// validateCode normalizes and validates a custom source code.
func validateCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", nil
	}
	if !codePattern.MatchString(code) {
		return "", fmt.Errorf("%w: invalid_code", platform.ErrValidation)
	}
	return code, nil
}

func (s *Service) linkURL(code string) string {
	base := s.WebOrigin
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/c/" + code
}

func (s *Service) totalsFor(ctx context.Context, q *repository.Queries, linkID string) (all SourceTotals, last30 SourceTotals, err error) {
	allRow, err := q.SumDailyLinkStatsAllTime(ctx, linkID)
	if err != nil {
		return all, last30, err
	}
	all = SourceTotals{Clicks: allRow.Clicks, UniqueClicks: allRow.UniqueClicks,
		Registrations: allRow.Registrations, FirstPayments: allRow.FirstPayments,
		IncomeKopecks: allRow.IncomeKopecks}

	period := tracking.LastDays(30)
	row, err := q.SumDailyLinkStats(ctx, repository.SumDailyLinkStatsParams{
		TrackingLinkID: linkID,
		Day:            repository.DatePtr(period.From),
		Day_2:          repository.DatePtr(period.To),
	})
	if err != nil {
		return all, last30, err
	}
	last30 = SourceTotals{Clicks: row.Clicks, UniqueClicks: row.UniqueClicks,
		Registrations: row.Registrations, FirstPayments: row.FirstPayments,
		IncomeKopecks: row.IncomeKopecks}
	return all, last30, nil
}

// sourceFromRow converts a listed link row (with joined group name) to Source.
func (s *Service) sourceFromRow(ctx context.Context, q *repository.Queries, r repository.ListTrackingLinksByAccessIDRow) (Source, error) {
	groupID := repository.UUIDToPtr(r.GroupID)
	groupName := repository.TextToPtr(r.GroupName)
	comment := repository.TextToPtr(r.Comment)
	all, last30, err := s.totalsFor(ctx, q, r.ID)
	if err != nil {
		return Source{}, err
	}
	var bonus *int
	if r.RegistrationBonus.Valid {
		v := int(r.RegistrationBonus.Int32)
		bonus = &v
	}
	var domain *string
	if r.Domain.Valid {
		domain = &r.Domain.String
	}
	var redirectID *string
	if r.RedirectID.Valid {
		s := r.RedirectID.String()
		redirectID = &s
	}
	return Source{
		ID: r.ID, Code: r.Code, Name: r.Name, Comment: comment,
		GroupID: groupID, GroupName: groupName, IsDefault: r.IsDefault, IsActive: r.IsActive,
		URL: s.linkURL(r.Code), Type: r.Type, RegistrationBonus: bonus, Domain: domain, RedirectID: redirectID,
		Totals: all, Totals30d: last30,
		CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
	}, nil
}

// ListSources returns all sources (tracking links) for a partner's offer with
// per-source totals.
func (s *Service) ListSources(ctx context.Context, partnerID, offerID string) ([]Source, error) {
	q := s.q(ctx)
	access, err := s.accessForOffer(ctx, q, partnerID, offerID)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListTrackingLinksByAccessID(ctx, access.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Source, 0, len(rows))
	for _, r := range rows {
		src, err := s.sourceFromRow(ctx, q, r)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}

// CreateSource adds a new source (custom tracking link) to a joined offer.
func (s *Service) CreateSource(ctx context.Context, partnerID, offerID, name string, code, comment, groupID *string, isDefault bool) (Source, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Source{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	q := s.q(ctx)
	access, err := s.accessForOffer(ctx, q, partnerID, offerID)
	if err != nil {
		return Source{}, err
	}
	normalized := ""
	if code != nil {
		if normalized, err = validateCode(*code); err != nil {
			return Source{}, err
		}
	}
	if normalized == "" {
		normalized, err = genLinkCode(ctx, q)
		if err != nil {
			return Source{}, err
		}
	}
	groupPg := repository.UUIDPtr(groupID)
	if groupID != nil && *groupID != "" {
		if _, err := s.ownedGroup(ctx, q, partnerID, *groupID); err != nil {
			return Source{}, err
		}
	}
	var link repository.TrackingLink
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		link, err = tq.CreateTrackingLink(ctx, repository.CreateTrackingLinkParams{
			PartnerOfferAccessID: access.ID,
			Code:                 normalized,
			Name:                 name,
			Comment:              repository.TextPtr(comment),
			GroupID:              groupPg,
			IsDefault:            false,
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
	return s.sourceByID(ctx, q, link.ID)
}

// UpdateSource mutates an existing source. code == nil/"" keeps the current code.
func (s *Service) UpdateSource(ctx context.Context, partnerID, offerID, sourceID, name string, code, comment, groupID *string, isActive, isDefault bool) (Source, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Source{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	q := s.q(ctx)
	link, access, err := s.ownedLink(ctx, q, partnerID, offerID, sourceID)
	if err != nil {
		return Source{}, err
	}
	normalized := link.Code
	if code != nil && strings.TrimSpace(*code) != "" {
		if normalized, err = validateCode(*code); err != nil {
			return Source{}, err
		}
	}
	if groupID != nil && *groupID != "" {
		if _, err := s.ownedGroup(ctx, q, partnerID, *groupID); err != nil {
			return Source{}, err
		}
	}
	// Last active source must remain active.
	if !isActive && link.IsActive {
		active, err := q.CountActiveLinksByAccess(ctx, access.ID)
		if err != nil {
			return Source{}, err
		}
		if active <= 1 {
			return Source{}, fmt.Errorf("%w: last_source", platform.ErrConflict)
		}
	}
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		if _, err := tq.UpdateTrackingLink(ctx, repository.UpdateTrackingLinkParams{
			ID:       link.ID,
			Name:     name,
			Code:     normalized,
			Comment:  repository.TextPtr(comment),
			GroupID:  repository.UUIDPtr(groupID),
			IsActive: isActive,
		}); err != nil {
			return err
		}
		if isDefault {
			if !link.IsDefault {
				if err := tq.ClearDefaultTrackingLinks(ctx, access.ID); err != nil {
					return err
				}
				return tq.SetDefaultTrackingLink(ctx, link.ID)
			}
			return nil
		}
		if link.IsDefault {
			return tq.ClearDefaultTrackingLinks(ctx, access.ID)
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Source{}, fmt.Errorf("%w: code_taken", platform.ErrConflict)
		}
		return Source{}, err
	}
	return s.sourceByID(ctx, q, link.ID)
}

// DeleteSource removes a source. Sources with clicks cannot be removed — use
// deactivation instead.
func (s *Service) DeleteSource(ctx context.Context, partnerID, offerID, sourceID string) error {
	q := s.q(ctx)
	link, access, err := s.ownedLink(ctx, q, partnerID, offerID, sourceID)
	if err != nil {
		return err
	}
	clicks, err := q.CountClicksByLink(ctx, link.ID)
	if err != nil {
		return err
	}
	if clicks > 0 {
		return fmt.Errorf("%w: has_clicks", platform.ErrConflict)
	}
	active, err := q.CountActiveLinksByAccess(ctx, access.ID)
	if err != nil {
		return err
	}
	if link.IsActive && active <= 1 {
		return fmt.Errorf("%w: last_source", platform.ErrConflict)
	}
	return q.DeleteTrackingLink(ctx, link.ID)
}

// sourceByID loads a full Source (with totals) after create/update.
func (s *Service) sourceByID(ctx context.Context, q *repository.Queries, id string) (Source, error) {
	row, err := q.ListTrackingLinksByAccessID(ctx, s.linkAccessID(ctx, q, id))
	if err != nil {
		return Source{}, err
	}
	for _, r := range row {
		if r.ID == id {
			return s.sourceFromRow(ctx, q, r)
		}
	}
	return Source{}, fmt.Errorf("%w: source_not_found", platform.ErrNotFound)
}

// linkAccessID resolves the access id of a tracking link.
func (s *Service) linkAccessID(ctx context.Context, q *repository.Queries, linkID string) string {
	link, err := q.GetTrackingLinkByID(ctx, linkID)
	if err != nil {
		return ""
	}
	return link.PartnerOfferAccessID
}

// ListGroups returns the partner's source groups.
func (s *Service) ListGroups(ctx context.Context, partnerID string) ([]Group, error) {
	q := s.q(ctx)
	rows, err := q.ListSourceGroupsByPartner(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, r := range rows {
		out = append(out, Group{
			ID: r.ID, Name: r.Name, Comment: repository.TextToPtr(r.Comment),
			CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
			UpdatedAt: r.UpdatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// CreateGroup adds a source group.
func (s *Service) CreateGroup(ctx context.Context, partnerID, name string, comment *string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	row, err := s.q(ctx).CreateSourceGroup(ctx, repository.CreateSourceGroupParams{
		PartnerID: partnerID, Name: name, Comment: repository.TextPtr(comment),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Group{}, fmt.Errorf("%w: group_name_taken", platform.ErrConflict)
		}
		return Group{}, err
	}
	return Group{
		ID: row.ID, Name: row.Name, Comment: repository.TextToPtr(row.Comment),
		CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}, nil
}

// UpdateGroup renames a group and updates its comment.
func (s *Service) UpdateGroup(ctx context.Context, partnerID, groupID, name string, comment *string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	q := s.q(ctx)
	if _, err := s.ownedGroup(ctx, q, partnerID, groupID); err != nil {
		return Group{}, err
	}
	row, err := q.UpdateSourceGroup(ctx, repository.UpdateSourceGroupParams{
		ID: groupID, Name: name, Comment: repository.TextPtr(comment),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Group{}, fmt.Errorf("%w: group_name_taken", platform.ErrConflict)
		}
		return Group{}, err
	}
	return Group{
		ID: row.ID, Name: row.Name, Comment: repository.TextToPtr(row.Comment),
		CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}, nil
}

// DeleteGroup removes an empty group.
func (s *Service) DeleteGroup(ctx context.Context, partnerID, groupID string) error {
	q := s.q(ctx)
	if _, err := s.ownedGroup(ctx, q, partnerID, groupID); err != nil {
		return err
	}
	count, err := q.CountSourcesInGroup(ctx, repository.UUIDPtr(&groupID))
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: group_not_empty", platform.ErrConflict)
	}
	return q.DeleteSourceGroup(ctx, groupID)
}

