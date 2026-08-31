package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"cashx/internal/partners"
	"cashx/internal/repository"
)

// CabinetConfig handles GET /cabinet/config - returns allowed domains and default domain.
func (s *Server) CabinetConfig(w http.ResponseWriter, r *http.Request) {
	domains, err := s.Offers.AllowedDomains(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	def, err := s.Offers.DefaultDomain(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"domains":       domains,
		"defaultDomain": def,
	})
}

// CabinetRegistrationBonus handles GET /cabinet/registration-bonus?ref=CODE
func (s *Server) CabinetRegistrationBonus(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	bonus, err := s.Offers.GetRegistrationBonus(r.Context(), ref)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"bonus": bonus})
}

// CabinetB2CReferrals handles GET /cabinet/b2c-referrals?from=&to=&search=&limit=&offset=
func (s *Server) CabinetB2CReferrals(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	if partnerID == "" {
		respond(w, http.StatusUnauthorized, errBody("unauthorized"))
		return
	}
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	from, to := partners.ParseRange(fromStr, toStr)
	total, sum, items, err := s.Partners.GetB2CReferrals(r.Context(), partnerID, from, to)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	if items == nil {
		items = []partners.B2CReferralItem{}
	}
	// Filter by search if provided
	if search != "" {
		low := strings.ToLower(search)
		filtered := make([]partners.B2CReferralItem, 0, len(items))
		for _, it := range items {
			if strings.Contains(strings.ToLower(it.Name), low) || strings.Contains(strings.ToLower(it.UserID), low) || strings.Contains(strings.ToLower(it.SourceName), low) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
		total = len(items)
		// recompute sum for filtered? Keep original sum for total, but for filtered we need sum of filtered income
		sum = 0
		for _, it := range items {
			sum += it.Income
		}
	}
	// Pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 50
	offset := 0
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	paged := items[offset:end]
	if paged == nil {
		paged = []partners.B2CReferralItem{}
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"sum":   sum,
		"items": paged,
	})
}

// CabinetB2CReferralsCSV handles GET /cabinet/b2c-referrals.csv
func (s *Server) CabinetB2CReferralsCSV(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	from, to := partners.ParseRange(fromStr, toStr)
	_, _, items, err := s.Partners.GetB2CReferrals(r.Context(), partnerID, from, to)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=b2c-referrals.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"user_id", "name", "kind", "source", "deposits_count", "deposits_sum", "income", "created_at"})
	for _, it := range items {
		_ = cw.Write([]string{
			it.UserID,
			it.Name,
			it.Kind,
			it.SourceName,
			strconv.FormatInt(it.DepositsCount, 10),
			strconv.FormatInt(it.DepositsSum, 10),
			strconv.FormatInt(it.Income, 10),
			it.CreatedAt,
		})
	}
	cw.Flush()
}

// CabinetLeaderboard handles GET /cabinet/leaderboard?period=&metric=
func (s *Server) CabinetLeaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	metric := r.URL.Query().Get("metric")
	items, err := s.Partners.GetLeaderboard(r.Context(), period, metric)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"period": period,
		"metric": metric,
		"items":  items,
	})
}

// CabinetTransactions handles GET /cabinet/transactions?limit=
func (s *Server) CabinetTransactions(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	wallet, err := s.Q.GetWalletByPartnerID(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	ledger, err := s.Q.ListLedgerByWallet(r.Context(), repository.ListLedgerByWalletParams{WalletID: wallet.ID, Limit: 200})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type tx struct {
		ID            string `json:"id"`
		Type          string `json:"type"`
		AmountKopecks int64  `json:"amount_kopecks"`
		CreatedAt     string `json:"created_at"`
	}
	items := []tx{}
	for _, e := range ledger {
		items = append(items, tx{
			ID: strconv.FormatInt(e.ID, 10), Type: e.Type, AmountKopecks: e.AmountKopecks,
			CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	respond(w, http.StatusOK, map[string]interface{}{"items": items})

	// CSV export if requested
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "type", "amount_kopecks", "created_at"})
		for _, it := range items {
			_ = cw.Write([]string{it.ID, it.Type, strconv.FormatInt(it.AmountKopecks, 10), it.CreatedAt})
		}
		cw.Flush()
	}
}

// CabinetAttrib handles POST /cabinet/attrib {ref, click_token}
func (s *Server) CabinetAttrib(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref        string `json:"ref"`
		ClickToken string `json:"click_token"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	ref := strings.TrimSpace(body.Ref)
	clickToken := strings.TrimSpace(body.ClickToken)
	if clickToken == "" {
		clickToken = r.Header.Get("X-Click-Token")
	}
	// Get user
	u := userFrom(r)
	if u == nil {
		respond(w, http.StatusUnauthorized, errBody("unauthorized"))
		return
	}
	// For now, try to attribute via tracking service: create external_user_attributions if ref is provided
	// We need project_id: pick first active project or offer's project
	// Simplified: find partner's first access project
	ok := false
	if ref != "" {
		// Try to find tracking link by code for attribution
		if link, err := s.Q.GetTrackingLinkByCodeExtended(r.Context(), strings.ToUpper(ref)); err == nil {
			// Find access and project
			access, err := s.Q.GetAccessByID(r.Context(), link.PartnerOfferAccessID)
			if err == nil {
				// Get offer to get project
				offer, err := s.Offers.Get(r.Context(), access.OfferID)
				if err == nil {
					// Need projectID from offer
					// Use raw query to get project_id via offer
					var projectID string
					err = s.Pool.QueryRow(r.Context(), `SELECT project_id FROM offers WHERE id=$1`, access.OfferID).Scan(&projectID)
					if err == nil {
						// Create attribution first-touch
						_, err = s.Q.CreateAttribution(r.Context(), repository.CreateAttributionParams{
							ProjectID:       projectID,
							TrackingClickID: repository.Int64ToPg(nil),
							PartnerID:       repository.UUIDPtr(&access.PartnerID),
							OfferID:         repository.UUIDPtr(&access.OfferID),
							ExternalUserID:  u.ID,
						})
						if err == nil || strings.Contains(err.Error(), "duplicate") {
							ok = true
						}
						_ = offer
					}
				}
			}
		}
	}
	// If click_token provided, also store? For now just return ok
	_ = clickToken
	respond(w, http.StatusOK, map[string]interface{}{"attributed": ok})
}

// AdminDomains handlers
func (s *Server) AdminDomainsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.Offers.ListDomains(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) AdminDomainCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL      string  `json:"url"`
		IsActive *bool   `json:"is_active"`
		Comment  *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.CreateDomain(r.Context(), body.URL, body.IsActive, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"domain": row})
}

func (s *Server) AdminDomainUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	var body struct {
		URL      *string `json:"url"`
		IsActive *bool   `json:"is_active"`
		Comment  *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.UpdateDomain(r.Context(), id, body.URL, body.IsActive, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"domain": row})
}

func (s *Server) AdminDomainDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Offers.DeleteDomain(r.Context(), id); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdminRedirects handlers
func (s *Server) AdminRedirectsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.Offers.ListRedirectPoolsWithURLs(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) AdminRedirectCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string   `json:"name"`
		Comment *string  `json:"comment"`
		URLs    []string `json:"urls"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.CreateRedirectPool(r.Context(), body.Name, body.Comment, body.URLs)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"redirect": row})
}

func (s *Server) AdminRedirectUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name    *string `json:"name"`
		Comment *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.UpdateRedirectPool(r.Context(), id, body.Name, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"redirect": row})
}

func (s *Server) AdminRedirectDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Offers.DeleteRedirectPool(r.Context(), id); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminRedirectAddURL(w http.ResponseWriter, r *http.Request) {
	redirectID := chi.URLParam(r, "id")
	var body struct {
		URL    string `json:"url"`
		Weight *int   `json:"weight"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.AddRedirectPoolURL(r.Context(), redirectID, body.URL, body.Weight)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"url": row})
}

func (s *Server) AdminRedirectUpdateURL(w http.ResponseWriter, r *http.Request) {
	redirectID := chi.URLParam(r, "id")
	urlID := chi.URLParam(r, "urlId")
	var body struct {
		URL      *string `json:"url"`
		Weight   *int    `json:"weight"`
		IsActive *bool   `json:"is_active"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.UpdateRedirectPoolURL(r.Context(), redirectID, urlID, body.URL, body.Weight, body.IsActive)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"url": row})
}

func (s *Server) AdminRedirectDeleteURL(w http.ResponseWriter, r *http.Request) {
	redirectID := chi.URLParam(r, "id")
	urlID := chi.URLParam(r, "urlId")
	if err := s.Offers.DeleteRedirectPoolURL(r.Context(), redirectID, urlID); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Helper to handle GetRedirectPoolByID not found mapping
func isNoRows(err error) bool { return err != nil && err.Error() == "no rows" || err == pgx.ErrNoRows }

// decodeBody helper already exists in other server files; ensure import
var _ = time.Now
