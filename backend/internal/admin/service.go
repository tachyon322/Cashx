// Package admin implements admin panel operations.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/audit"
	"cashx/internal/auth"
	"cashx/internal/notifications"
	"cashx/internal/platform"
	"cashx/internal/repository"
)

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Service implements admin operations over partners and platform content.
type Service struct {
	Pool      *pgxpool.Pool
	Auth      *auth.Service
	Limen     *auth.Limen
	Audit     *audit.Recorder
	WebOrigin string
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// PartnerRow is one row of the admin partner list.
type PartnerRow struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	IsApproved         bool   `json:"is_approved"`
	IsBlocked          bool   `json:"is_blocked"`
	BalanceKopecks     int64  `json:"balance_kopecks"`
	RevsharePercentBps int    `json:"revshare_percent_bps"`
	Rates              []Rate `json:"rates"`
	CreatedAt          string `json:"created_at"`
}

// Rate is a partner's personal rate for an offer.
type Rate struct {
	OfferID string `json:"offer_id"`
	RateBps int    `json:"rate_bps"`
}

// ListPartners returns the admin partner list.
func (s *Service) ListPartners(ctx context.Context, search, status string, limit, offset int) ([]PartnerRow, int64, error) {
	q := s.q(ctx)
	rows, err := q.ListPartnerProfilesAdmin(ctx, repository.ListPartnerProfilesAdminParams{
		Column1: search, Column2: status, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountPartnerProfilesAdmin(ctx, repository.CountPartnerProfilesAdminParams{Column1: search, Column2: status})
	if err != nil {
		return nil, 0, err
	}
	// All access rates grouped by partner (admin pages are small).
	allRates, err := q.ListAccessRatesAll(ctx)
	if err != nil {
		return nil, 0, err
	}
	ratesByPartner := make(map[string][]Rate)
	for _, r := range allRates {
		ratesByPartner[r.PartnerID] = append(ratesByPartner[r.PartnerID], Rate{OfferID: r.OfferID, RateBps: int(r.RateBps)})
	}
	out := make([]PartnerRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, PartnerRow{
			ID: r.ID, Name: r.Name, Email: r.Email,
			IsApproved: r.IsApproved, IsBlocked: r.IsBlocked,
			BalanceKopecks: r.AvailableKopecks + r.ReservedKopecks,
			RevsharePercentBps: int(r.RevsharePercentBps),
			Rates:              ratesByPartner[r.ID],
			CreatedAt:          r.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return out, total, nil
}

// CreatePartner creates an approved partner with a wallet.
func (s *Service) CreatePartner(ctx context.Context, actorID *string, name, email, password string, commissionBps *int) (repository.User, error) {
	user, err := s.Auth.Register(ctx, name, email, password, "")
	if err != nil {
		return repository.User{}, err
	}
	profile, err := s.q(ctx).GetPartnerProfileByUserID(ctx, user.ID)
	if err != nil {
		return repository.User{}, err
	}
	if _, err := s.q(ctx).UpdatePartnerProfile(ctx, repository.UpdatePartnerProfileParams{ID: profile.ID, IsApproved: repository.BoolPtr(boolPtr(true))}); err != nil {
		return repository.User{}, err
	}
	// The partner's single RevShare percent applies to every offer they join.
	if commissionBps != nil {
		if *commissionBps < 0 || *commissionBps > 10000 {
			return repository.User{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
		}
		if err := s.q(ctx).SetPartnerRevShare(ctx, repository.SetPartnerRevShareParams{
			ID: profile.ID, RevsharePercentBps: int32(*commissionBps),
		}); err != nil {
			return repository.User{}, err
		}
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "partner.created", "partner", profile.ID, map[string]any{"email": email}, nil)
	}
	return *user, nil
}

// UpdatePartner patches approval/block/name/email/password/revshare.
func (s *Service) UpdatePartner(ctx context.Context, actorID *string, id string, name, email, password *string, isApproved, isBlocked *bool, revshareBps *int) (PartnerRow, error) {
	q := s.q(ctx)
	profile, err := q.GetPartnerProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PartnerRow{}, fmt.Errorf("%w: partner_not_found", platform.ErrNotFound)
		}
		return PartnerRow{}, err
	}
	wasApproved := profile.IsApproved
	if email != nil && !emailRe.MatchString(strings.ToLower(strings.TrimSpace(*email))) {
		return PartnerRow{}, fmt.Errorf("%w: invalid_email", platform.ErrValidation)
	}
	if name != nil && strings.TrimSpace(*name) == "" {
		return PartnerRow{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if revshareBps != nil && (*revshareBps < 0 || *revshareBps > 10000) {
		return PartnerRow{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
	}
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		if name != nil || email != nil || isBlocked != nil {
			if _, err := tq.UpdateUser(ctx, repository.UpdateUserParams{
				ID: profile.UserID, Name: repository.TextPtr(name), Email: repository.TextPtr(email),
			}); err != nil {
				return err
			}
		}
		if isApproved != nil || isBlocked != nil || revshareBps != nil {
			if _, err := tq.UpdatePartnerProfile(ctx, repository.UpdatePartnerProfileParams{
				ID: profile.ID, IsApproved: repository.BoolPtr(isApproved), IsBlocked: repository.BoolPtr(isBlocked),
				RevsharePercentBps: repository.Int32Ptr(revshareBps),
			}); err != nil {
				return err
			}
		}
		if revshareBps != nil {
			// Sync all existing accesses to the new global revshare.
			if err := tq.UpdateAllAccessRatesByPartner(ctx, repository.UpdateAllAccessRatesByPartnerParams{
				PartnerID: profile.ID, RateBps: int32(*revshareBps),
			}); err != nil {
				return err
			}
		}
		if password != nil && *password != "" {
			if len(*password) < 8 {
				return fmt.Errorf("%w: invalid_password", platform.ErrValidation)
			}
			hash, err := s.Limen.Password.HashPassword(*password)
			if err != nil {
				return err
			}
			if err := tq.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{ID: profile.UserID, Password: repository.TextPtr(&hash)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return PartnerRow{}, fmt.Errorf("%w: email_taken", platform.ErrConflict)
		}
		return PartnerRow{}, err
	}
	// A changed password invalidates every existing session.
	if password != nil && *password != "" {
		if err := s.Limen.RevokeAllSessions(ctx, profile.UserID); err != nil {
			return PartnerRow{}, err
		}
	}
	// Notify on first approval.
	if isApproved != nil && *isApproved && !wasApproved {
		_ = notifications.NotifyUser(ctx, q, profile.UserID, "announcement", "Аккаунт одобрен", "Добро пожаловать в партнёрскую программу CashX", nil)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "partner.updated", "partner", id, map[string]any{
			"is_approved": isApproved, "is_blocked": isBlocked, "name": name, "email": email, "revshare_bps": revshareBps,
		}, nil)
	}
	row, err := s.GetPartner(ctx, id)
	if err != nil {
		return PartnerRow{}, err
	}
	return row, nil
}

// SetRate upserts the partner's personal rate for an offer.
func (s *Service) SetRate(ctx context.Context, actorID *string, partnerID, offerID string, rateBps int) (Rate, error) {
	if rateBps < 0 || rateBps > 10000 {
		return Rate{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
	}
	row, err := s.q(ctx).UpdatePartnerAccessRate(ctx, repository.UpdatePartnerAccessRateParams{
		PartnerID: partnerID, OfferID: offerID, RateBps: int32(rateBps),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Partner is not joined to this offer: create the access without a link.
			access, err := s.q(ctx).CreatePartnerAccess(ctx, repository.CreatePartnerAccessParams{
				PartnerID: partnerID, OfferID: offerID, RateBps: int32(rateBps),
			})
			if err != nil {
				return Rate{}, err
			}
			return Rate{OfferID: access.OfferID, RateBps: int(access.RateBps)}, nil
		}
		return Rate{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "partner.rate_changed", "partner", partnerID, map[string]any{"offer_id": offerID, "rate_bps": rateBps}, nil)
	}
	return Rate{OfferID: row.OfferID, RateBps: int(row.RateBps)}, nil
}

// GetPartner returns the full partner detail.
func (s *Service) GetPartner(ctx context.Context, id string) (PartnerRow, error) {
	q := s.q(ctx)
	profile, err := q.GetPartnerProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PartnerRow{}, fmt.Errorf("%w: partner_not_found", platform.ErrNotFound)
		}
		return PartnerRow{}, err
	}
	user, err := q.GetUserByPartnerID(ctx, id)
	if err != nil {
		return PartnerRow{}, err
	}
	wallet, err := q.GetWalletByPartnerID(ctx, id)
	if err != nil {
		return PartnerRow{}, err
	}
	rates, err := q.ListPartnerAccesses(ctx, id)
	if err != nil {
		return PartnerRow{}, err
	}
	out := PartnerRow{
		ID: profile.ID, Name: user.Name, Email: user.Email,
		IsApproved: profile.IsApproved, IsBlocked: profile.IsBlocked,
		BalanceKopecks: wallet.AvailableKopecks + wallet.ReservedKopecks,
		RevsharePercentBps: int(profile.RevsharePercentBps),
		CreatedAt:          profile.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	for _, r := range rates {
		out.Rates = append(out.Rates, Rate{OfferID: r.OfferID, RateBps: int(r.RateBps)})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Announcements.

// Announcement is the admin announcement shape.
type Announcement struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	Audience    string  `json:"audience"`
	IsPublished bool    `json:"is_published"`
	PublishedAt *string `json:"published_at"`
	DeletedAt   *string `json:"deleted_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func annRow(a repository.Announcement) Announcement {
	return Announcement{
		ID: a.ID, Title: a.Title, Body: a.Body, Audience: a.Audience,
		IsPublished: a.IsPublished,
		PublishedAt: timePtrStr(a.PublishedAt), DeletedAt: timePtrStr(a.DeletedAt),
		CreatedAt: a.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func timePtrStr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}

// CreateAnnouncement inserts an announcement (optionally published).
func (s *Service) CreateAnnouncement(ctx context.Context, actorID *string, title, body, audience string, partnerIDs []string, publish bool) (Announcement, error) {
	title = strings.TrimSpace(title)
	if title == "" || body == "" {
		return Announcement{}, fmt.Errorf("%w: invalid_announcement", platform.ErrValidation)
	}
	if audience != "all" && audience != "partners" && audience != "staff" && audience != "specific_partner" {
		return Announcement{}, fmt.Errorf("%w: invalid_audience", platform.ErrValidation)
	}
	if audience == "specific_partner" && len(partnerIDs) == 0 {
		return Announcement{}, fmt.Errorf("%w: partner_ids_required", platform.ErrValidation)
	}
	var out Announcement
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		row, err := tq.InsertAnnouncement(ctx, repository.InsertAnnouncementParams{
			Title: title, Body: body, Audience: audience, CreatedBy: repository.UUIDPtr(actorID),
		})
		if err != nil {
			return err
		}
		if audience == "specific_partner" {
			for _, pid := range partnerIDs {
				if err := tq.InsertAnnouncementAudience(ctx, repository.InsertAnnouncementAudienceParams{AnnouncementID: row.ID, PartnerID: repository.UUIDPtr(&pid)}); err != nil {
					return err
				}
			}
		}
		if publish {
			row, err = tq.PublishAnnouncement(ctx, row.ID)
			if err != nil {
				return err
			}
		}
		out = annRow(row)
		return nil
	})
	if err != nil {
		return Announcement{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "announcement.created", "announcement", out.ID, map[string]any{"audience": audience}, nil)
	}
	return out, nil
}

// UpdateAnnouncement patches title/body/audience and optionally republishes.
func (s *Service) UpdateAnnouncement(ctx context.Context, actorID *string, id string, title, body, audience *string, partnerIDs []string, publish *bool) (Announcement, error) {
	q := s.q(ctx)
	if _, err := q.GetAnnouncement(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Announcement{}, fmt.Errorf("%w: announcement_not_found", platform.ErrNotFound)
		}
		return Announcement{}, err
	}
	var out Announcement
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		row, err := tq.UpdateAnnouncement(ctx, repository.UpdateAnnouncementParams{
			ID: id, Title: repository.TextPtr(title), Body: repository.TextPtr(body), Audience: repository.TextPtr(audience),
		})
		if err != nil {
			return err
		}
		if partnerIDs != nil {
			if err := tq.ReplaceAnnouncementAudiences(ctx, id); err != nil {
				return err
			}
			for _, pid := range partnerIDs {
				if err := tq.InsertAnnouncementAudience(ctx, repository.InsertAnnouncementAudienceParams{AnnouncementID: id, PartnerID: repository.UUIDPtr(&pid)}); err != nil {
					return err
				}
			}
		}
		if publish != nil && *publish {
			row, err = tq.PublishAnnouncement(ctx, id)
			if err != nil {
				return err
			}
		}
		out = annRow(row)
		return nil
	})
	if err != nil {
		return Announcement{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "announcement.updated", "announcement", id, map[string]any{"title": title}, nil)
	}
	return out, nil
}

// DeleteAnnouncement soft-deletes an announcement.
func (s *Service) DeleteAnnouncement(ctx context.Context, actorID *string, id string) error {
	if _, err := s.q(ctx).GetAnnouncement(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: announcement_not_found", platform.ErrNotFound)
		}
		return err
	}
	if err := s.q(ctx).SoftDeleteAnnouncement(ctx, id); err != nil {
		return err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "announcement.deleted", "announcement", id, nil, nil)
	}
	return nil
}

// ListAnnouncements returns all announcements including deleted.
func (s *Service) ListAnnouncements(ctx context.Context) ([]Announcement, error) {
	rows, err := s.q(ctx).ListAnnouncements(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Announcement, 0, len(rows))
	for _, r := range rows {
		out = append(out, annRow(r))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Branding.

// Branding is the platform branding settings.
type Branding struct {
	Name        string  `json:"name"`
	TelegramURL *string `json:"telegram_url"`
	AvatarURL   *string `json:"avatar_url"`
}

// GetBranding reads platform_settings.branding.
func (s *Service) GetBranding(ctx context.Context) (Branding, error) {
	raw, err := platform.GetJSONSetting(ctx, s.q(ctx), "branding")
	if err != nil || raw == nil {
		return Branding{Name: "CashX"}, err
	}
	var b Branding
	if err := json.Unmarshal(raw, &b); err != nil {
		return Branding{Name: "CashX"}, err
	}
	return b, nil
}

// UpdateBranding writes platform_settings.branding.
func (s *Service) UpdateBranding(ctx context.Context, actorID *string, name string, telegramURL *string, avatarURL *string) (Branding, error) {
	if strings.TrimSpace(name) == "" {
		return Branding{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if avatarURL != nil && strings.TrimSpace(*avatarURL) == "" {
		avatarURL = nil
	}
	b := Branding{Name: name, TelegramURL: telegramURL, AvatarURL: avatarURL}
	raw, err := json.Marshal(b)
	if err != nil {
		return Branding{}, err
	}
	if _, err := s.q(ctx).SetPlatformSetting(ctx, repository.SetPlatformSettingParams{Key: "branding", Value: raw}); err != nil {
		return Branding{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "branding.updated", "branding", "singleton", map[string]any{"name": name}, nil)
	}
	return b, nil
}

func boolPtr(b bool) *bool { return &b }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
