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
	"github.com/thecodearcher/limen"
	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

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

// ---------------------------------------------------------------------------
// Staff management — only superadmin can manage.

var allowedStaffRoles = map[string]bool{
	"superadmin":      true,
	"project_manager": true,
	"finance":         true,
	"content_manager": true,
	"support":         true,
}

// StaffMember is the admin staff list/detail shape.
type StaffMember struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	IsActive  bool     `json:"is_active"`
	Roles     []string `json:"roles"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func validateStaffRoles(roles []string) error {
	if len(roles) == 0 {
		return fmt.Errorf("%w: roles_required", platform.ErrValidation)
	}
	seen := map[string]bool{}
	for _, r := range roles {
		if !allowedStaffRoles[r] {
			return fmt.Errorf("%w: invalid_role %s", platform.ErrValidation, r)
		}
		if seen[r] {
			return fmt.Errorf("%w: duplicate_role %s", platform.ErrValidation, r)
		}
		seen[r] = true
	}
	return nil
}

func staffFromRow(u repository.User, roles []string) StaffMember {
	return StaffMember{
		ID: u.ID, Email: u.Email, Name: u.Name, IsActive: u.IsActive,
		Roles: roles,
		CreatedAt: u.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

// ListStaff returns paginated staff members with optional search.
func (s *Service) ListStaff(ctx context.Context, search string, limit, offset int) ([]StaffMember, int64, error) {
	search = strings.TrimSpace(search)
	like := "%" + search + "%"
	var total int64
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role='staff' AND ($1='' OR email ILIKE $2 OR name ILIKE $2)`, search, like).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, email, name, role, is_active, created_at, updated_at FROM users WHERE role='staff' AND ($1='' OR email ILIKE $2 OR name ILIKE $2) ORDER BY created_at DESC LIMIT $3 OFFSET $4`, search, like, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []StaffMember
	for rows.Next() {
		var u repository.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		// load roles for this user
		rrows, err := s.Pool.Query(ctx, `SELECT role FROM staff_role_assignments WHERE user_id=$1 ORDER BY role`, u.ID)
		if err != nil {
			return nil, 0, err
		}
		var roles []string
		for rrows.Next() {
			var r string
			if err := rrows.Scan(&r); err != nil {
				rrows.Close()
				return nil, 0, err
			}
			roles = append(roles, r)
		}
		rrows.Close()
		if roles == nil {
			roles = []string{}
		}
		out = append(out, staffFromRow(u, roles))
	}
	if out == nil {
		out = []StaffMember{}
	}
	return out, total, nil
}

// GetStaff returns a single staff member by user ID.
func (s *Service) GetStaff(ctx context.Context, id string) (StaffMember, error) {
	var u repository.User
	err := s.Pool.QueryRow(ctx, `SELECT id, email, name, role, is_active, created_at, updated_at FROM users WHERE id=$1 AND role='staff'`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StaffMember{}, fmt.Errorf("%w: staff_not_found", platform.ErrNotFound)
		}
		return StaffMember{}, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT role FROM staff_role_assignments WHERE user_id=$1 ORDER BY role`, id)
	if err != nil {
		return StaffMember{}, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return StaffMember{}, err
		}
		roles = append(roles, r)
	}
	if roles == nil {
		roles = []string{}
	}
	return staffFromRow(u, roles), nil
}

// CreateStaff creates a new staff user with the given roles. Only superadmin should call this.
func (s *Service) CreateStaff(ctx context.Context, actorID *string, name, email, password string, roles []string) (StaffMember, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return StaffMember{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRe.MatchString(email) {
		return StaffMember{}, fmt.Errorf("%w: invalid_email", platform.ErrValidation)
	}
	if len(password) < 8 {
		return StaffMember{}, fmt.Errorf("%w: invalid_password", platform.ErrValidation)
	}
	if err := validateStaffRoles(roles); err != nil {
		return StaffMember{}, err
	}
	pwd := password
	res, err := s.Limen.Password.SignUpWithCredentialAndPassword(ctx,
		&limen.User{Email: email, Password: &pwd},
		map[string]any{"name": name, "role": "staff", "is_active": true},
	)
	if err != nil {
		if errors.Is(err, credentialpassword.ErrEmailAlreadyExists) {
			// Promote existing user (partner -> staff) instead of failing.
			var existingID, existingRole string
			if err2 := s.Pool.QueryRow(ctx, `SELECT id, role FROM users WHERE email=$1`, email).Scan(&existingID, &existingRole); err2 != nil {
				return StaffMember{}, fmt.Errorf("%w: email_taken", platform.ErrConflict)
			}
			// Update role to staff, name and password if provided
			if _, err2 := s.Pool.Exec(ctx, `UPDATE users SET role='staff', name=$2, updated_at=now() WHERE id=$1`, existingID, name); err2 != nil {
				return StaffMember{}, err2
			}
			hash, err2 := s.Limen.Password.HashPassword(password)
			if err2 != nil {
				return StaffMember{}, err2
			}
			if _, err2 := s.Pool.Exec(ctx, `UPDATE users SET password=$2, updated_at=now() WHERE id=$1`, existingID, hash); err2 != nil {
				return StaffMember{}, err2
			}
			for _, r := range roles {
				if _, err2 := s.Pool.Exec(ctx, `INSERT INTO staff_role_assignments (user_id, role, project_id) VALUES ($1,$2,NULL) ON CONFLICT DO NOTHING`, existingID, r); err2 != nil {
					return StaffMember{}, err2
				}
			}
			if s.Audit != nil {
				_ = s.Audit.Record(ctx, actorID, "staff.promoted", "staff", existingID, map[string]any{"email": email, "roles": roles}, nil)
			}
			return s.GetStaff(ctx, existingID)
		}
		return StaffMember{}, err
	}
	userID := fmt.Sprint(res.User.ID)
	// insert roles
	for _, r := range roles {
		if _, err := s.Pool.Exec(ctx, `INSERT INTO staff_role_assignments (user_id, role, project_id) VALUES ($1,$2,NULL) ON CONFLICT DO NOTHING`, userID, r); err != nil {
			return StaffMember{}, err
		}
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "staff.created", "staff", userID, map[string]any{"email": email, "roles": roles}, nil)
	}
	return s.GetStaff(ctx, userID)
}

// UpdateStaff patches a staff member. Nil pointers mean no change. Roles when non-nil replaces all.
func (s *Service) UpdateStaff(ctx context.Context, actorID *string, id string, name, email, password *string, isActive *bool, roles *[]string) (StaffMember, error) {
	// existence + current roles check
	current, err := s.GetStaff(ctx, id)
	if err != nil {
		return StaffMember{}, err
	}
	if email != nil {
		*email = strings.ToLower(strings.TrimSpace(*email))
		if !emailRe.MatchString(*email) {
			return StaffMember{}, fmt.Errorf("%w: invalid_email", platform.ErrValidation)
		}
	}
	if name != nil && strings.TrimSpace(*name) == "" {
		return StaffMember{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if password != nil && *password != "" && len(*password) < 8 {
		return StaffMember{}, fmt.Errorf("%w: invalid_password", platform.ErrValidation)
	}
	if roles != nil {
		if err := validateStaffRoles(*roles); err != nil {
			return StaffMember{}, err
		}
		// protect last superadmin
		hasSuper := false
		for _, r := range current.Roles {
			if r == "superadmin" {
				hasSuper = true
				break
			}
		}
		willHaveSuper := false
		for _, r := range *roles {
			if r == "superadmin" {
				willHaveSuper = true
				break
			}
		}
		if hasSuper && !willHaveSuper {
			var cnt int64
			if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM staff_role_assignments WHERE role='superadmin'`).Scan(&cnt); err != nil {
				return StaffMember{}, err
			}
			if cnt <= 1 {
				return StaffMember{}, fmt.Errorf("%w: last_superadmin", platform.ErrValidation)
			}
		}
	}
	if isActive != nil && !*isActive {
		hasSuper := false
		for _, r := range current.Roles {
			if r == "superadmin" {
				hasSuper = true
				break
			}
		}
		if hasSuper {
			var cnt int64
			if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM staff_role_assignments WHERE role='superadmin'`).Scan(&cnt); err != nil {
				return StaffMember{}, err
			}
			if cnt <= 1 {
				return StaffMember{}, fmt.Errorf("%w: last_superadmin", platform.ErrValidation)
			}
		}
	}
	// Use a manual transaction so user + role changes are atomic.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return StaffMember{}, err
	}
	defer tx.Rollback(ctx)
	tq := repository.New(tx)
	if name != nil || email != nil || isActive != nil {
		if _, err := tq.UpdateUser(ctx, repository.UpdateUserParams{
			ID: id, Name: repository.TextPtr(name), Email: repository.TextPtr(email), IsActive: repository.BoolPtr(isActive),
		}); err != nil {
			if isUniqueViolation(err) {
				return StaffMember{}, fmt.Errorf("%w: email_taken", platform.ErrConflict)
			}
			return StaffMember{}, err
		}
	}
	if roles != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM staff_role_assignments WHERE user_id=$1`, id); err != nil {
			return StaffMember{}, err
		}
		for _, r := range *roles {
			if _, err := tx.Exec(ctx, `INSERT INTO staff_role_assignments (user_id, role, project_id) VALUES ($1,$2,NULL)`, id, r); err != nil {
				return StaffMember{}, err
			}
		}
	}
	if password != nil && *password != "" {
		hash, err := s.Limen.Password.HashPassword(*password)
		if err != nil {
			return StaffMember{}, err
		}
		if err := tq.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{ID: id, Password: repository.TextPtr(&hash)}); err != nil {
			return StaffMember{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		if isUniqueViolation(err) {
			return StaffMember{}, fmt.Errorf("%w: email_taken", platform.ErrConflict)
		}
		return StaffMember{}, err
	}
	if password != nil && *password != "" {
		_ = s.Limen.RevokeAllSessions(ctx, id)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "staff.updated", "staff", id, map[string]any{"name": name, "email": email, "is_active": isActive, "roles": roles}, nil)
	}
	return s.GetStaff(ctx, id)
}

func boolPtr(b bool) *bool { return &b }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
