package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"cashx/internal/api/gen"
	"cashx/internal/auth"
	"cashx/internal/offers"
	"cashx/internal/payouts"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// CabinetSummary handles GET /cabinet/summary.
func (s *Server) CabinetSummary(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	if partnerID == "" {
		respond(w, http.StatusUnauthorized, errBody("unauthorized"))
		return
	}
	summary, err := s.Partners.GetSummary(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		Balance balance `json:"balance"`
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
		Chart        []dayStats `json:"chart"`
		ActiveOffers []struct {
			OfferID       string `json:"offer_id"`
			Name          string `json:"name"`
			RateBps       int    `json:"rate_bps"`
			TrackingURL   string `json:"tracking_url"`
			Clicks        int64  `json:"clicks"`
			Registrations int64  `json:"registrations"`
			IncomeKopecks int64  `json:"income_kopecks"`
		} `json:"active_offers"`
		RevsharePercentBps int `json:"revshare_percent_bps"`
		Telegram           struct {
			Connected bool `json:"connected"`
		} `json:"telegram"`
	}{
		Balance: balance{AvailableKopecks: summary.Balance.AvailableKopecks, ReservedKopecks: summary.Balance.ReservedKopecks},
		Chart:   chartToDayStats(summary.Chart),
		Telegram: struct {
			Connected bool `json:"connected"`
		}{Connected: summary.Telegram.Connected},
		RevsharePercentBps: summary.RevsharePercentBps,
	}
	resp.Income.TodayKopecks = summary.Income.TodayKopecks
	resp.Income.WeekKopecks = summary.Income.WeekKopecks
	resp.Income.MonthKopecks = summary.Income.MonthKopecks
	resp.Income.AllKopecks = summary.Income.AllKopecks
	resp.Funnel.Clicks = summary.Funnel.Clicks
	resp.Funnel.UniqueClicks = summary.Funnel.UniqueClicks
	resp.Funnel.Registrations = summary.Funnel.Registrations
	resp.Funnel.FirstPayments = summary.Funnel.FirstPayments
	resp.Funnel.IncomeKopecks = summary.Funnel.IncomeKopecks
	for _, o := range summary.ActiveOffers {
		resp.ActiveOffers = append(resp.ActiveOffers, struct {
			OfferID       string `json:"offer_id"`
			Name          string `json:"name"`
			RateBps       int    `json:"rate_bps"`
			TrackingURL   string `json:"tracking_url"`
			Clicks        int64  `json:"clicks"`
			Registrations int64  `json:"registrations"`
			IncomeKopecks int64  `json:"income_kopecks"`
		}{OfferID: o.OfferID, Name: o.Name, RateBps: o.RateBps, TrackingURL: o.TrackingURL,
			Clicks: o.Clicks, Registrations: o.Registrations, IncomeKopecks: o.IncomeKopecks})
	}
	respond(w, http.StatusOK, resp)
}

// CabinetOffersList handles GET /cabinet/offers.
func (s *Server) CabinetOffersList(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	items, err := s.Partners.ListOffers(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type cabinetOffer struct {
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
	out := struct {
		Items []cabinetOffer `json:"items"`
	}{}
	for _, it := range items {
		out.Items = append(out.Items, cabinetOffer{
			OfferID: it.OfferID, ProjectID: it.ProjectID, ProjectName: it.ProjectName,
			ProjectLogoURL: it.ProjectLogoURL, Name: it.Name, Category: it.Category,
			Description: it.Description, Status: it.Status, MyRateBps: it.MyRateBps,
			EPC: it.EPC, CR: it.CR, MyTrackingURL: it.MyTrackingURL,
		})
	}
	respond(w, http.StatusOK, out)
}

// CabinetOfferJoin handles POST /cabinet/offers/{offerId}/join.
func (s *Server) CabinetOfferJoin(w http.ResponseWriter, r *http.Request, offerId string) {
	partnerID := partnerIDFrom(r)
	rate, trackingURL, err := s.Offers.Join(r.Context(), partnerID, offerId)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, struct {
		OfferId     string `json:"offer_id"`
		RateBps     int    `json:"rate_bps"`
		TrackingUrl string `json:"tracking_url"`
	}{OfferId: offerId, RateBps: rate, TrackingUrl: trackingURL})
}

// CabinetOfferStats handles GET /cabinet/offers/{offerId}.
func (s *Server) CabinetOfferStats(w http.ResponseWriter, r *http.Request, offerId string) {
	partnerID := partnerIDFrom(r)
	resp, err := s.offerStats(r.Context(), partnerID, offerId)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, resp)
}

// CabinetOfferHistoryCSV serves the CSV export (outside OpenAPI).
func (s *Server) CabinetOfferHistoryCSV(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	offerId := r.PathValue("offerId")
	if offerId == "" {
		respond(w, http.StatusNotFound, errBody("offer_not_found"))
		return
	}
	from := parseTimeParam(r.URL.Query().Get("from"), time.Now().AddDate(0, -3, 0))
	to := parseTimeParam(r.URL.Query().Get("to"), time.Now())
	rows, err := s.offerHistory(r.Context(), partnerID, offerId, from, to, 5000)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=history.csv")
	_, _ = w.Write([]byte("date,kind,amount_kopecks\n"))
	for _, h := range rows {
		amt := ""
		if h.AmountKopecks != nil {
			amt = strconv.FormatInt(*h.AmountKopecks, 10)
		}
		_, _ = w.Write([]byte(h.OccurredAt.Format(time.RFC3339) + "," + h.Kind + "," + amt + "\n"))
	}
}

// CabinetPayoutsConfig handles GET /cabinet/payouts/config.
func (s *Server) CabinetPayoutsConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Payouts.GetConfig(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, payoutRulesFromConfig(cfg))
}

// CabinetPayoutsList handles GET /cabinet/payouts.
func (s *Server) CabinetPayoutsList(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	wallet, err := s.Q.GetWalletByPartnerID(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	requests, err := s.Payouts.ListByPartner(r.Context(), partnerID, 200)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	ledger, err := s.Q.ListLedgerByWallet(r.Context(), repository.ListLedgerByWalletParams{WalletID: wallet.ID, Limit: 200})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		Balance  balance               `json:"balance"`
		Requests []withdrawalResponse  `json:"requests"`
		History  []ledgerEntryResponse `json:"history"`
	}{
		Balance:  balance{AvailableKopecks: wallet.AvailableKopecks, ReservedKopecks: wallet.ReservedKopecks},
		Requests: []withdrawalResponse{},
		History:  []ledgerEntryResponse{},
	}
	for _, req := range requests {
		resp.Requests = append(resp.Requests, withdrawalResponseFrom(req))
	}
	for _, e := range ledger {
		resp.History = append(resp.History, ledgerEntryResponseFrom(e))
	}
	respond(w, http.StatusOK, resp)
}

// CabinetPayoutRequest handles POST /cabinet/payouts/requests.
func (s *Server) CabinetPayoutRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Method        string `json:"method"`
		AmountKopecks int64  `json:"amount_kopecks"`
		Requisites    string `json:"requisites"`
		Bank          string `json:"bank"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	partnerID := partnerIDFrom(r)
	row, err := s.Payouts.Request(r.Context(), partnerID, body.Method, body.AmountKopecks, body.Requisites, body.Bank)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, withdrawalResponseFrom(row))
}

// CabinetPayoutCancel handles POST /cabinet/payouts/requests/{id}/cancel.
func (s *Server) CabinetPayoutCancel(w http.ResponseWriter, r *http.Request, id string) {
	partnerID := partnerIDFrom(r)
	row, err := s.Payouts.Cancel(r.Context(), partnerID, id)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, withdrawalResponseFrom(row))
}

// CabinetReferrals handles GET /cabinet/referrals.
func (s *Server) CabinetReferrals(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	info, err := s.Partners.GetReferrals(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		ReferralCode       string `json:"referral_code"`
		InviteURL          string `json:"invite_url"`
		TotalInvited       int64  `json:"total_invited"`
		TotalRewardKopecks int64  `json:"total_reward_kopecks"`
		Items              []struct {
			PartnerID     string  `json:"partner_id"`
			Name          string  `json:"name"`
			Email         *string `json:"email"`
			JoinedAt      string  `json:"joined_at"`
			RewardKopecks int64   `json:"reward_kopecks"`
		} `json:"items"`
	}{
		ReferralCode: info.ReferralCode, InviteURL: info.InviteURL,
		TotalInvited: info.TotalInvited, TotalRewardKopecks: info.TotalRewardKopecks,
	}
	for _, it := range info.Items {
		resp.Items = append(resp.Items, struct {
			PartnerID     string  `json:"partner_id"`
			Name          string  `json:"name"`
			Email         *string `json:"email"`
			JoinedAt      string  `json:"joined_at"`
			RewardKopecks int64   `json:"reward_kopecks"`
		}{PartnerID: it.PartnerID, Name: it.Name, Email: it.Email, JoinedAt: it.JoinedAt, RewardKopecks: it.RewardKopecks})
	}
	respond(w, http.StatusOK, resp)
}

// CabinetNotificationsList handles GET /cabinet/notifications.
func (s *Server) CabinetNotificationsList(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	partnerID := partnerIDFrom(r)
	items, unread, err := s.Partners.GetNotifications(r.Context(), u.ID, partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		UnreadCount int `json:"unread_count"`
		Items       []struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Title     string `json:"title"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
			Read      bool   `json:"read"`
		} `json:"items"`
	}{UnreadCount: unread}
	for _, it := range items {
		resp.Items = append(resp.Items, struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Title     string `json:"title"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
			Read      bool   `json:"read"`
		}{ID: it.ID, Type: it.Type, Title: it.Title, Body: it.Body, CreatedAt: it.CreatedAt, Read: it.Read})
	}
	respond(w, http.StatusOK, resp)
}

// CabinetNotificationsReadAll handles POST /cabinet/notifications/read-all.
func (s *Server) CabinetNotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	partnerID := partnerIDFrom(r)
	if err := s.Partners.MarkAllRead(r.Context(), u.ID, partnerID); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CabinetNotificationRead handles POST /cabinet/notifications/{type}/{id}/read.
func (s *Server) CabinetNotificationRead(w http.ResponseWriter, r *http.Request, pType gen.CabinetNotificationReadParamsType, id string) {
	u := userFrom(r)
	partnerID := partnerIDFrom(r)
	if err := s.Partners.MarkOneRead(r.Context(), u.ID, partnerID, string(pType), id); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CabinetProfileUpdate handles PUT /cabinet/profile.
func (s *Server) CabinetProfileUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           *string `json:"name"`
		TelegramUserId *int64  `json:"telegram_user_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	u := userFrom(r)
	if err := s.Partners.UpdateProfile(r.Context(), u.ID, body.Name, body.TelegramUserId); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		User userResponse `json:"user"`
	}{User: userResponseFrom(u)})
}

// ---------------------------------------------------------------------------
// Shared response shapes and conversions.

type balance struct {
	AvailableKopecks int64 `json:"available_kopecks"`
	ReservedKopecks  int64 `json:"reserved_kopecks"`
}

type dayStats struct {
	Date          string `json:"date"`
	Clicks        int64  `json:"clicks"`
	UniqueClicks  int64  `json:"unique_clicks"`
	Registrations int64  `json:"registrations"`
	FirstPayments int64  `json:"first_payments"`
	IncomeKopecks int64  `json:"income_kopecks"`
}

type withdrawalResponse struct {
	ID            string   `json:"id"`
	PartnerID     string   `json:"partner_id"`
	AmountKopecks int64    `json:"amount_kopecks"`
	Method        string   `json:"method"`
	Requisites    string   `json:"requisites"`
	Bank          *string  `json:"bank"`
	FeeKopecks    int64    `json:"fee_kopecks"`
	UsdtAmount    *float64 `json:"usdt_amount"`
	Rate          *float64 `json:"rate"`
	Status        string   `json:"status"`
	Comment       *string  `json:"comment"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func withdrawalResponseFrom(w repository.WithdrawalRequest) withdrawalResponse {
	out := withdrawalResponse{
		ID: w.ID, PartnerID: w.PartnerID, AmountKopecks: w.AmountKopecks,
		Method: w.Method, Requisites: w.Requisites, Bank: repository.TextToPtr(w.Bank),
		FeeKopecks: w.FeeKopecks, Status: w.Status, Comment: repository.TextToPtr(w.Comment),
		CreatedAt: w.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt: w.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if w.UsdtAmount.Valid {
		f := repository.NumToFloat64(w.UsdtAmount)
		out.UsdtAmount = &f
	}
	if w.Rate.Valid {
		f := repository.NumToFloat64(w.Rate)
		out.Rate = &f
	}
	return out
}

type ledgerEntryResponse struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	AmountKopecks       int64  `json:"amount_kopecks"`
	BalanceAfterKopecks int64  `json:"balance_after_kopecks"`
	CreatedAt           string `json:"created_at"`
}

func ledgerEntryResponseFrom(e repository.WalletLedgerEntry) ledgerEntryResponse {
	return ledgerEntryResponse{
		ID: strconv.FormatInt(e.ID, 10), Type: e.Type,
		AmountKopecks: e.AmountKopecks, BalanceAfterKopecks: e.BalanceAfterKopecks,
		CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

type payoutRulesResponse struct {
	MinWithdrawKopecks int64   `json:"min_withdraw_kopecks"`
	UsdtRate           float64 `json:"usdt_rate"`
	SbpFeeFlatKopecks  int64   `json:"sbp_fee_flat_kopecks"`
	SbpFeePercentBps   int     `json:"sbp_fee_percent_bps"`
	UpdatedAt          string  `json:"updated_at"`
}

func payoutRulesFromConfig(cfg payouts.Config) payoutRulesResponse {
	return payoutRulesResponse{
		MinWithdrawKopecks: cfg.MinWithdrawKopecks, UsdtRate: cfg.UsdtRate,
		SbpFeeFlatKopecks: cfg.SbpFeeFlatKopecks, SbpFeePercentBps: cfg.SbpFeePercentBps,
		UpdatedAt: cfg.UpdatedAt,
	}
}

type userResponse struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	IsApproved bool   `json:"is_approved"`
	IsBlocked  bool   `json:"is_blocked"`
	Partner    *struct {
		ReferralCode   string `json:"referral_code"`
		TelegramUserId *int64 `json:"telegram_user_id"`
	} `json:"partner"`
	Staff *struct {
		Roles []string `json:"roles"`
	} `json:"staff"`
}

func userResponseFrom(u *auth.User) userResponse {
	out := userResponse{
		ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role,
		IsApproved: u.Partner != nil && u.Partner.IsApproved,
		IsBlocked:  u.Partner != nil && u.Partner.IsBlocked,
	}
	if u.Partner != nil {
		out.Partner = &struct {
			ReferralCode   string `json:"referral_code"`
			TelegramUserId *int64 `json:"telegram_user_id"`
		}{ReferralCode: u.Partner.ReferralCode, TelegramUserId: u.Partner.TelegramUserID}
	}
	if len(u.Staff) > 0 {
		roles := make([]string, 0, len(u.Staff))
		for _, r := range u.Staff {
			roles = append(roles, r.Role)
		}
		out.Staff = &struct {
			Roles []string `json:"roles"`
		}{Roles: roles}
	}
	return out
}

type offerAggResponse struct {
	IncomeKopecks int64 `json:"income_kopecks"`
	Clicks        int64 `json:"clicks"`
	UniqueClicks  int64 `json:"unique_clicks"`
	Registrations int64 `json:"registrations"`
	FirstPayments int64 `json:"first_payments"`
}

type offerStatsResponse struct {
	Offer       offerCardResponse `json:"offer"`
	TrackingURL string            `json:"tracking_url"`
	Summary     struct {
		Today offerAggResponse `json:"today"`
		Week  offerAggResponse `json:"week"`
		Month offerAggResponse `json:"month"`
		All   offerAggResponse `json:"all"`
	} `json:"summary"`
	Chart   []dayStats    `json:"chart"`
	History []historyItem `json:"history"`
}

type historyItem struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	AmountKopecks *int64 `json:"amount_kopecks"`
	OccurredAt    string `json:"occurred_at"`
}

type offerCardResponse struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"project_id"`
	ProjectName    string  `json:"project_name"`
	Name           string  `json:"name"`
	Category       *string `json:"category"`
	Description    *string `json:"description"`
	DestinationURL *string `json:"destination_url"`
	Status         string  `json:"status"`
	CurrentRateBps int     `json:"current_rate_bps"`
	Version        int     `json:"version"`
	CreatedAt      string  `json:"created_at"`
}

func offerCardResponseFrom(c offers.Card) offerCardResponse {
	return offerCardResponse{
		ID: c.ID, ProjectID: c.ProjectID, ProjectName: c.ProjectName,
		Name: c.Name, Category: c.Category, Description: c.Description,
		DestinationURL: c.DestinationURL, Status: c.Status,
		CurrentRateBps: c.CurrentRateBps, Version: c.Version, CreatedAt: c.CreatedAt,
	}
}

// offerStats builds the offer statistics response.
func (s *Server) offerStats(ctx context.Context, partnerID, offerID string) (offerStatsResponse, error) {
	access, err := s.Q.GetPartnerAccess(ctx, repository.GetPartnerAccessParams{PartnerID: partnerID, OfferID: offerID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || (err != nil && err.Error() == "no rows in result set") {
			return offerStatsResponse{}, fmt.Errorf("%w: offer_not_joined", platform.ErrNotFound)
		}
		return offerStatsResponse{}, err
	}
	if access.Status != "active" {
		return offerStatsResponse{}, fmt.Errorf("%w: offer_not_joined", platform.ErrNotFound)
	}
	card, err := s.Offers.Get(ctx, offerID)
	if err != nil {
		return offerStatsResponse{}, err
	}
	agg := func(p tracking.Period) offerAggResponse {
		t, err := tracking.TotalsOffer(ctx, s.Q, partnerID, offerID, p)
		if err != nil {
			return offerAggResponse{}
		}
		return offerAggResponse{IncomeKopecks: t.IncomeKopecks, Clicks: t.Clicks,
			UniqueClicks: t.UniqueClicks, Registrations: t.Registrations, FirstPayments: t.FirstPayments}
	}
	allT, err := tracking.TotalsOfferAllTime(ctx, s.Q, partnerID, offerID)
	if err != nil {
		return offerStatsResponse{}, err
	}
	resp := offerStatsResponse{Offer: offerCardResponseFrom(card)}

	// Personal tracking link (default source), always available for a joined offer.
	if link, err := s.Q.GetDefaultTrackingLinkByAccessID(ctx, access.ID); err == nil {
		base := s.Cfg.WebOrigin
		if base == "" {
			base = "http://localhost:3000"
		}
		resp.TrackingURL = base + "/c/" + link.Code
	}
	resp.Summary.Today = agg(tracking.Today())
	resp.Summary.Week = agg(tracking.LastDays(7))
	resp.Summary.Month = agg(tracking.LastDays(30))
	resp.Summary.All = offerAggResponse{IncomeKopecks: allT.IncomeKopecks, Clicks: allT.Clicks,
		UniqueClicks: allT.UniqueClicks, Registrations: allT.Registrations, FirstPayments: allT.FirstPayments}
	chart, err := tracking.DailyByOffer(ctx, s.Q, partnerID, offerID, tracking.LastDays(30))
	if err != nil {
		return offerStatsResponse{}, err
	}
	for _, d := range chart {
		resp.Chart = append(resp.Chart, dayStats(d))
	}
	rows, err := s.offerHistory(ctx, partnerID, offerID, time.Now().AddDate(0, -3, 0), time.Now(), 200)
	if err != nil {
		return offerStatsResponse{}, err
	}
	for _, h := range rows {
		resp.History = append(resp.History, historyItem{
			ID: h.ID, Kind: h.Kind, AmountKopecks: h.AmountKopecks,
			OccurredAt: h.OccurredAt.UTC().Format(time.RFC3339),
		})
	}
	return resp, nil
}

// HistoryRow is one entry of the mixed history feed.
type HistoryRow struct {
	ID            string
	Kind          string
	AmountKopecks *int64
	OccurredAt    time.Time
}

// offerHistory assembles the mixed history feed for an offer.
func (s *Server) offerHistory(ctx context.Context, partnerID, offerID string, from, to time.Time, limit int) ([]HistoryRow, error) {
	access, err := s.Q.GetPartnerAccess(ctx, repository.GetPartnerAccessParams{PartnerID: partnerID, OfferID: offerID})
	if err != nil {
		return nil, err
	}
	link, err := s.Q.GetDefaultTrackingLinkByAccessID(ctx, access.ID)
	if err != nil {
		return nil, err
	}
	var out []HistoryRow
	clicks, err := s.Q.HistoryClicksByLink(ctx, repository.HistoryClicksByLinkParams{
		TrackingLinkID: link.ID, CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to), Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	for _, c := range clicks {
		out = append(out, HistoryRow{ID: "click_" + strconv.FormatInt(c.ID, 10), Kind: "click", OccurredAt: c.CreatedAt.Time})
	}
	attrs, err := s.Q.HistoryAttributionsByLink(ctx, repository.HistoryAttributionsByLinkParams{
		TrackingLinkID: link.ID, FirstSeenAt: repository.TimePtr(&from), FirstSeenAt_2: repository.TimePtr(&to), Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	for _, a := range attrs {
		out = append(out, HistoryRow{ID: "reg_" + strconv.FormatInt(a.ID, 10), Kind: "registration", OccurredAt: a.FirstSeenAt.Time})
	}
	convs, err := s.Q.HistoryConversionsByLink(ctx, repository.HistoryConversionsByLinkParams{
		TrackingLinkID: link.ID, OccurredAt: repository.TimePtr(&from), OccurredAt_2: repository.TimePtr(&to), Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	for _, c := range convs {
		amt := c.AmountKopecks
		out = append(out, HistoryRow{ID: "pay_" + strconv.FormatInt(c.ID, 10), Kind: "payment", AmountKopecks: &amt, OccurredAt: c.OccurredAt.Time})
	}
	earnings, err := s.Q.HistoryEarningsByPartnerOffer(ctx, repository.HistoryEarningsByPartnerOfferParams{
		PartnerID: partnerID, OfferID: offerID, CreatedAt: repository.TimePtr(&from), CreatedAt_2: repository.TimePtr(&to), Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	for _, e := range earnings {
		amt := e.AmountKopecks
		kind := "earning"
		if e.ReversedAt.Valid {
			kind = "reversal"
			amt = -amt
		}
		out = append(out, HistoryRow{ID: "earn_" + e.ID, Kind: kind, AmountKopecks: &amt, OccurredAt: e.CreatedAt.Time})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OccurredAt.After(out[j].OccurredAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func parseTimeParam(raw string, def time.Time) time.Time {
	if raw == "" {
		return def
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return def
	}
	return t
}

func chartToDayStats(in []tracking.DayStats) []dayStats {
	out := make([]dayStats, 0, len(in))
	for _, d := range in {
		out = append(out, dayStats(d))
	}
	return out
}
