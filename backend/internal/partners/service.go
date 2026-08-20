// Package partners implements the partner cabinet domain.
package partners

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/notifications"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// Service provides cabinet operations for the authenticated partner.
type Service struct {
	Pool      *pgxpool.Pool
	WebOrigin string
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// Summary is the dashboard response.
type Summary struct {
	Balance Balance `json:"balance"`
	Income  struct {
		TodayKopecks int64 `json:"today_kopecks"`
		WeekKopecks  int64 `json:"week_kopecks"`
		MonthKopecks int64 `json:"month_kopecks"`
		AllKopecks   int64 `json:"all_kopecks"`
	} `json:"income"`
	Funnel struct {
		Clicks        int64 `json:"clicks"`
		UniqueClicks  int64 `json:"unique_clicks"`
		Registrations int64 `json:"registrations"`
		FirstPayments int64 `json:"first_payments"`
		IncomeKopecks int64 `json:"income_kopecks"`
	} `json:"funnel"`
	Chart              []tracking.DayStats `json:"chart"`
	ActiveOffers       []ActiveOffer       `json:"active_offers"`
	RevsharePercentBps int                 `json:"revshare_percent_bps"`
	Telegram           struct {
		Connected bool `json:"connected"`
	} `json:"telegram"`
}

// Balance is the wallet state.
type Balance struct {
	AvailableKopecks int64 `json:"available_kopecks"`
	ReservedKopecks  int64 `json:"reserved_kopecks"`
}

// ActiveOffer is one joined offer in the summary.
type ActiveOffer struct {
	OfferID       string `json:"offer_id"`
	Name          string `json:"name"`
	RateBps       int    `json:"rate_bps"`
	TrackingURL   string `json:"tracking_url"`
	Clicks        int64  `json:"clicks"`
	Registrations int64  `json:"registrations"`
	IncomeKopecks int64  `json:"income_kopecks"`
}

// GetSummary builds the dashboard.
func (s *Service) GetSummary(ctx context.Context, partnerID string) (Summary, error) {
	q := s.q(ctx)
	var out Summary

	wallet, err := q.GetWalletByPartnerID(ctx, partnerID)
	if err != nil {
		return out, err
	}
	out.Balance = Balance{AvailableKopecks: wallet.AvailableKopecks, ReservedKopecks: wallet.ReservedKopecks}

	today, _ := tracking.TotalsFor(ctx, q, partnerID, tracking.Today())
	week, _ := tracking.TotalsFor(ctx, q, partnerID, tracking.LastDays(7))
	month, _ := tracking.TotalsFor(ctx, q, partnerID, tracking.LastDays(30))
	all, err := tracking.TotalsAllTime(ctx, q, partnerID)
	if err != nil {
		return out, err
	}
	out.Income.TodayKopecks = today.IncomeKopecks
	out.Income.WeekKopecks = week.IncomeKopecks
	out.Income.MonthKopecks = month.IncomeKopecks
	out.Income.AllKopecks = all.IncomeKopecks
	out.Funnel.Clicks = all.Clicks
	out.Funnel.UniqueClicks = all.UniqueClicks
	out.Funnel.Registrations = all.Registrations
	out.Funnel.FirstPayments = all.FirstPayments
	out.Funnel.IncomeKopecks = all.IncomeKopecks

	// 60 дней: чтобы на фронте можно было посчитать рост «месяц к прошлому месяцу».
	chart, err := tracking.Daily(ctx, q, partnerID, tracking.LastDays(60))
	if err != nil {
		return out, err
	}
	out.Chart = chart

	// Active offers with per-offer aggregates.
	accesses, err := q.ListPartnerAccessesWithOffer(ctx, partnerID)
	if err != nil {
		return out, err
	}
	allPeriod := tracking.AllTime()
	for _, a := range accesses {
		if a.OfferStatus != "active" {
			continue
		}
		t, err := tracking.TotalsOfferAllTime(ctx, q, partnerID, a.OfferID)
		if err != nil {
			return out, err
		}
		link, err := q.GetDefaultTrackingLinkByAccessID(ctx, a.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		url := ""
		if err == nil {
			url = s.trackingURL(link.Code)
		}
		out.ActiveOffers = append(out.ActiveOffers, ActiveOffer{
			OfferID: a.OfferID, Name: a.OfferName, RateBps: int(a.RateBps),
			TrackingURL: url, Clicks: t.Clicks, Registrations: t.Registrations,
			IncomeKopecks: t.IncomeKopecks,
		})
	}
	_ = allPeriod

	profile, err := q.GetPartnerProfileByID(ctx, partnerID)
	if err != nil {
		return out, err
	}
	out.RevsharePercentBps = int(profile.RevsharePercentBps)
	out.Telegram.Connected = profile.TelegramUserID.Valid
	return out, nil
}

// OfferListItem is a catalog row for the partner.
type OfferListItem struct {
	OfferID        string   `json:"offer_id"`
	ProjectID      string   `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	ProjectLogoURL *string  `json:"project_logo_url"`
	Name           string   `json:"name"`
	Category       *string  `json:"category"`
	Description    *string  `json:"description"`
	Status         string   `json:"status"`
	MyRateBps      *int     `json:"my_rate_bps"`
	EPC            *float64 `json:"epc"`
	CR             *float64 `json:"cr"`
	MyTrackingURL  *string  `json:"my_tracking_url"`
}

// ListOffers returns the offer catalog with the partner's access state.
func (s *Service) ListOffers(ctx context.Context, partnerID string) ([]OfferListItem, error) {
	q := s.q(ctx)
	rows, err := q.ListOffers(ctx, repository.ListOffersParams{Column1: "", Limit: 500, Offset: 0})
	if err != nil {
		return nil, err
	}
	accesses, err := q.ListPartnerAccessesWithOffer(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	accessByOffer := make(map[string]repository.ListPartnerAccessesWithOfferRow, len(accesses))
	for _, a := range accesses {
		accessByOffer[a.OfferID] = a
	}
	// EPC/CR over the last 30 days from daily stats.
	period := tracking.LastDays(30)
	out := make([]OfferListItem, 0, len(rows))
	for _, r := range rows {
		if r.Status == "pending" {
			continue
		}
		item := OfferListItem{
			OfferID: r.ID, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
			Name: r.Name, Category: repository.TextToPtr(r.Category),
			Description: repository.TextToPtr(r.Description), Status: r.Status,
		}
		if a, ok := accessByOffer[r.ID]; ok {
			rate := int(a.RateBps)
			item.MyRateBps = &rate
			if link, err := q.GetDefaultTrackingLinkByAccessID(ctx, a.ID); err == nil {
				url := s.trackingURL(link.Code)
				item.MyTrackingURL = &url
			}
			t, err := tracking.TotalsOffer(ctx, q, partnerID, r.ID, period)
			if err == nil {
				if t.Clicks > 0 {
					epc := float64(t.IncomeKopecks) / float64(t.Clicks)
					item.EPC = &epc
				}
				if t.Clicks > 0 {
					cr := float64(t.Registrations) / float64(t.Clicks) * 100
					item.CR = &cr
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// ReferralsInfo is the referrals screen payload.
type ReferralsInfo struct {
	ReferralCode       string         `json:"referral_code"`
	InviteURL          string         `json:"invite_url"`
	TotalInvited       int64          `json:"total_invited"`
	TotalRewardKopecks int64          `json:"total_reward_kopecks"`
	Items              []ReferralItem `json:"items"`
}

// ReferralItem is one invited partner with their contribution.
type ReferralItem struct {
	PartnerID     string  `json:"partner_id"`
	Name          string  `json:"name"`
	Email         *string `json:"email"`
	JoinedAt      string  `json:"joined_at"`
	RewardKopecks int64   `json:"reward_kopecks"`
}

// GetReferrals returns the partner's referral program state.
func (s *Service) GetReferrals(ctx context.Context, partnerID string) (ReferralsInfo, error) {
	q := s.q(ctx)
	var out ReferralsInfo
	profile, err := q.GetPartnerProfileByID(ctx, partnerID)
	if err != nil {
		return out, err
	}
	out.ReferralCode = profile.ReferralCode
	out.InviteURL = s.WebOrigin + "/invite/" + profile.ReferralCode
	total, err := q.CountReferralsByReferrer(ctx, partnerID)
	if err != nil {
		return out, err
	}
	out.TotalInvited = total
	rewards, err := q.SumRewardsByReferrer(ctx, partnerID)
	if err != nil {
		return out, err
	}
	out.TotalRewardKopecks = rewards
	rows, err := q.ListReferralsByReferrer(ctx, partnerID)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		reward, err := q.SumRewardsByInvited(ctx, r.InvitedPartnerID)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, ReferralItem{
			PartnerID: r.InvitedPartnerID, Name: r.Name,
			Email:         &r.Email,
			JoinedAt:      r.CreatedAt.Time.UTC().Format(time.RFC3339),
			RewardKopecks: reward,
		})
	}
	return out, nil
}

// NotificationsItem is one row in the bell dropdown.
type NotificationsItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Read      bool   `json:"read"`
}

// GetNotifications returns the bell list and unread counter.
func (s *Service) GetNotifications(ctx context.Context, userID, partnerID string) (items []NotificationsItem, unread int, err error) {
	q := s.q(ctx)
	unreadN, err := q.CountUnreadUserNotifications(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	personal, err := q.ListUserNotifications(ctx, repository.ListUserNotificationsParams{UserID: userID, Limit: 50})
	if err != nil {
		return nil, 0, err
	}
	announcements, err := q.ListVisibleAnnouncements(ctx)
	if err != nil {
		return nil, 0, err
	}
	for _, n := range personal {
		read := n.ReadAt.Valid
		items = append(items, NotificationsItem{
			ID: n.ID, Type: "personal", Title: n.Title, Body: n.Body,
			CreatedAt: n.CreatedAt.Time.UTC().Format(time.RFC3339), Read: read,
		})
	}
	// Announcement reads are joined per reader; partner-specific audience
	// visibility is resolved via announcement_audiences.
	for _, a := range announcements {
		if a.Audience == "specific_partner" {
			ids, err := q.ListAnnouncementAudiencePartnerIDs(ctx, a.ID)
			if err != nil {
				return nil, 0, err
			}
			visible := false
			for _, pid := range ids {
				if pid.Valid && pid.String() == partnerID {
					visible = true
					break
				}
			}
			if !visible {
				continue
			}
		} else if a.Audience == "staff" {
			continue
		}
		read := false
		if _, err := q.GetAnnouncementRead(ctx, repository.GetAnnouncementReadParams{AnnouncementID: a.ID, ReaderUserID: userID}); err == nil {
			read = true
		}
		items = append(items, NotificationsItem{
			ID: a.ID, Type: "announcement", Title: a.Title, Body: a.Body,
			CreatedAt: a.PublishedAt.Time.UTC().Format(time.RFC3339), Read: read,
		})
		if !read {
			unreadN++
		}
	}
	return items, int(unreadN), nil
}

// MarkAllRead marks personal notifications and visible announcements read.
func (s *Service) MarkAllRead(ctx context.Context, userID, partnerID string) error {
	q := s.q(ctx)
	if err := q.MarkAllUserNotificationsRead(ctx, userID); err != nil {
		return err
	}
	return q.MarkVisibleAnnouncementsRead(ctx, repository.MarkVisibleAnnouncementsReadParams{
		Column1: userID, Column2: partnerID,
	})
}

// MarkOneRead marks a single notification or announcement read.
func (s *Service) MarkOneRead(ctx context.Context, userID, partnerID, notifType, id string) error {
	q := s.q(ctx)
	switch notifType {
	case "personal":
		return q.MarkUserNotificationRead(ctx, repository.MarkUserNotificationReadParams{ID: id, UserID: userID})
	case "announcement":
		return q.InsertAnnouncementRead(ctx, repository.InsertAnnouncementReadParams{AnnouncementID: id, ReaderUserID: userID})
	default:
		return fmt.Errorf("%w: invalid_type", platform.ErrValidation)
	}
}

// UpdateProfile updates the partner's display name and Telegram id.
func (s *Service) UpdateProfile(ctx context.Context, userID string, name *string, telegramUserID *int64) error {
	q := s.q(ctx)
	profile, err := q.GetPartnerProfileByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if name != nil && *name == "" {
		return fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if telegramUserID != nil && *telegramUserID <= 0 {
		return fmt.Errorf("%w: invalid_telegram", platform.ErrValidation)
	}
	_, err = q.UpdateUser(ctx, repository.UpdateUserParams{ID: userID, Name: repository.TextPtr(name)})
	if err != nil {
		return err
	}
	_, err = q.UpdatePartnerProfile(ctx, repository.UpdatePartnerProfileParams{
		ID: profile.ID, TelegramUserID: repository.Int64ToPg(telegramUserID),
	})
	return err
}

func (s *Service) trackingURL(code string) string {
	base := s.WebOrigin
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/c/" + code
}

// Notify is a thin wrapper for notifications used by admin flows.
func Notify(ctx context.Context, q *repository.Queries, userID, notifType, title, body string) error {
	return notifications.NotifyUser(ctx, q, userID, notifType, title, body, nil)
}
