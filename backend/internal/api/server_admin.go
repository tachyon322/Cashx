package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cashx/internal/api/gen"
	"cashx/internal/admin"
	"cashx/internal/offers"
	"cashx/internal/platform"
	"cashx/internal/projects"
	"cashx/internal/repository"
)

func pagination(r *http.Request) (limit, offset int) {
	limit = platform.IntParam(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}
	offset = platform.IntParam(r, "offset", 0)
	return limit, offset
}

// AdminPartnersList handles GET /admin/partners.
func (s *Server) AdminPartnersList(w http.ResponseWriter, r *http.Request, params gen.AdminPartnersListParams) {
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	limit, offset := pagination(r)
	items, total, err := s.Admin.ListPartners(r.Context(), search, status, limit, offset)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		Total int64        `json:"total"`
		Items []adminPartner `json:"items"`
	}{Total: total}
	for _, it := range items {
		resp.Items = append(resp.Items, adminPartnerFrom(it))
	}
	respond(w, http.StatusOK, resp)
}

type adminPartner struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Email         string        `json:"email"`
	IsApproved    bool          `json:"is_approved"`
	IsBlocked     bool          `json:"is_blocked"`
	BalanceKopecks int64        `json:"balance_kopecks"`
	Rates         []struct {
		OfferID string `json:"offer_id"`
		RateBps int    `json:"rate_bps"`
	} `json:"rates"`
	CreatedAt string `json:"created_at"`
}

func adminPartnerFrom(p admin.PartnerRow) adminPartner {
	out := adminPartner{
		ID: p.ID, Name: p.Name, Email: p.Email,
		IsApproved: p.IsApproved, IsBlocked: p.IsBlocked,
		BalanceKopecks: p.BalanceKopecks, CreatedAt: p.CreatedAt,
	}
	for _, r := range p.Rates {
		out.Rates = append(out.Rates, struct {
			OfferID string `json:"offer_id"`
			RateBps int    `json:"rate_bps"`
		}{OfferID: r.OfferID, RateBps: r.RateBps})
	}
	return out
}

// adminPartnerRow aliases the admin service row to avoid import cycles in this file.
// AdminPartnerCreate handles POST /admin/partners.
func (s *Server) AdminPartnerCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		CommissionBps *int   `json:"commission_bps"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	user, err := s.Admin.CreatePartner(r.Context(), actorID(r), body.Name, body.Email, body.Password, body.CommissionBps)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}{Id: user.ID, Name: user.Name, Email: user.Email})
}

// AdminPartnerGet handles GET /admin/partners/{id}.
func (s *Server) AdminPartnerGet(w http.ResponseWriter, r *http.Request, id string) {
	partner, err := s.Admin.GetPartner(r.Context(), id)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	wallet, walletErr := s.Q.GetWalletByPartnerID(r.Context(), id)
	walletID := ""
	if walletErr == nil {
		walletID = wallet.ID
	}
	ledger, _ := s.Q.ListLedgerByWallet(r.Context(), repository.ListLedgerByWalletParams{WalletID: walletID, Limit: 200})
	withdrawals, _ := s.Payouts.ListByPartner(r.Context(), id, 200)
	accesses, _ := s.Q.ListPartnerAccessesWithOffer(r.Context(), id)

	resp := struct {
		Partner adminPartner `json:"partner"`
		Balance struct {
			AvailableKopecks int64 `json:"available_kopecks"`
			ReservedKopecks  int64 `json:"reserved_kopecks"`
		} `json:"balance"`
		Ledger      []ledgerEntryResponse  `json:"ledger"`
		Withdrawals []withdrawalResponse   `json:"withdrawals"`
		Accesses    []struct {
			OfferID   string `json:"offer_id"`
			OfferName string `json:"offer_name"`
			RateBps   int    `json:"rate_bps"`
			Status    string `json:"status"`
		} `json:"accesses"`
	}{Partner: adminPartnerFrom(partner)}
	if walletErr == nil {
		resp.Balance.AvailableKopecks = wallet.AvailableKopecks
		resp.Balance.ReservedKopecks = wallet.ReservedKopecks
	}
	for _, e := range ledger {
		resp.Ledger = append(resp.Ledger, ledgerEntryResponseFrom(e))
	}
	for _, w := range withdrawals {
		resp.Withdrawals = append(resp.Withdrawals, withdrawalResponseFrom(w))
	}
	for _, a := range accesses {
		resp.Accesses = append(resp.Accesses, struct {
			OfferID   string `json:"offer_id"`
			OfferName string `json:"offer_name"`
			RateBps   int    `json:"rate_bps"`
			Status    string `json:"status"`
		}{OfferID: a.OfferID, OfferName: a.OfferName, RateBps: int(a.RateBps), Status: a.Status})
	}
	respond(w, http.StatusOK, resp)
}

// AdminPartnerUpdate handles PATCH /admin/partners/{id}.
func (s *Server) AdminPartnerUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name       *string `json:"name"`
		Email      *string `json:"email"`
		Password   *string `json:"password"`
		IsApproved *bool   `json:"is_approved"`
		IsBlocked  *bool   `json:"is_blocked"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Admin.UpdatePartner(r.Context(), actorID(r), id, body.Name, body.Email, body.Password, body.IsApproved, body.IsBlocked)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, adminPartnerFrom(row))
}

// AdminPartnerRate handles POST /admin/partners/{id}/rate.
func (s *Server) AdminPartnerRate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		OfferID string `json:"offer_id"`
		RateBps int    `json:"rate_bps"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	rate, err := s.Admin.SetRate(r.Context(), actorID(r), id, body.OfferID, body.RateBps)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		OfferID string `json:"offer_id"`
		RateBps int    `json:"rate_bps"`
	}{OfferID: rate.OfferID, RateBps: rate.RateBps})
}

// AdminProjectsList handles GET /admin/projects.
func (s *Server) AdminProjectsList(w http.ResponseWriter, r *http.Request, params gen.AdminProjectsListParams) {
	limit, offset := pagination(r)
	items, total, err := s.Projects.List(r.Context(), limit, offset)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		Total int64          `json:"total"`
		Items []projectCard  `json:"items"`
	}{Total: total, Items: projectCards(items)})
}

type projectCard struct {
	ID             string  `json:"id"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	DestinationURL string  `json:"destination_url"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
}

func projectCards(items []projects.Card) []projectCard {
	out := make([]projectCard, 0, len(items))
	for _, it := range items {
		out = append(out, projectCard{
			ID: it.ID, Slug: it.Slug, Name: it.Name, Description: it.Description,
			DestinationURL: it.DestinationURL, IsActive: it.IsActive, CreatedAt: it.CreatedAt,
		})
	}
	return out
}

// AdminProjectCreate handles POST /admin/projects.
func (s *Server) AdminProjectCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug           string `json:"slug"`
		Name           string `json:"name"`
		Description    string `json:"description"`
		DestinationURL string `json:"destination_url"`
		IsActive       *bool  `json:"is_active"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	active := body.IsActive == nil || *body.IsActive
	card, err := s.Projects.Create(r.Context(), actorID(r), body.Slug, body.Name, body.Description, body.DestinationURL, active)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, projectCardFromService(card))
}

func projectCardFromService(c projects.Card) projectCard {
	return projectCard{
		ID: c.ID, Slug: c.Slug, Name: c.Name, Description: c.Description,
		DestinationURL: c.DestinationURL, IsActive: c.IsActive, CreatedAt: c.CreatedAt,
	}
}

// AdminProjectUpdate handles PATCH /admin/projects/{id}.
func (s *Server) AdminProjectUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name           *string `json:"name"`
		Description    *string `json:"description"`
		DestinationURL *string `json:"destination_url"`
		IsActive       *bool   `json:"is_active"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	card, err := s.Projects.Update(r.Context(), actorID(r), id, body.Name, body.Description, body.DestinationURL, body.IsActive)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, projectCardFromService(card))
}

// AdminOffersList handles GET /admin/offers.
func (s *Server) AdminOffersList(w http.ResponseWriter, r *http.Request, params gen.AdminOffersListParams) {
	projectID := r.URL.Query().Get("project_id")
	limit, offset := pagination(r)
	items, total, err := s.Offers.List(r.Context(), projectID, limit, offset)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	out := make([]offerCardResponse, 0, len(items))
	for _, it := range items {
		out = append(out, offerCardResponseFrom(it))
	}
	respond(w, http.StatusOK, struct {
		Total int64               `json:"total"`
		Items []offerCardResponse `json:"items"`
	}{Total: total, Items: out})
}

// AdminOfferCreate handles POST /admin/offers.
func (s *Server) AdminOfferCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID      string  `json:"project_id"`
		Name           string  `json:"name"`
		Category       *string `json:"category"`
		Description    *string `json:"description"`
		DestinationURL *string `json:"destination_url"`
		Status         *string `json:"status"`
		RateBps        int     `json:"rate_bps"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	status := ""
	if body.Status != nil {
		status = *body.Status
	}
	card, err := s.Offers.Create(r.Context(), actorID(r), body.ProjectID, body.Name, body.Category, body.Description, body.DestinationURL, status, body.RateBps)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, offerCardResponseFrom(card))
}

// AdminOfferUpdate handles PATCH /admin/offers/{id}.
func (s *Server) AdminOfferUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name           *string `json:"name"`
		Category       *string `json:"category"`
		Description    *string `json:"description"`
		DestinationURL *string `json:"destination_url"`
		Status         *string `json:"status"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	card, err := s.Offers.Update(r.Context(), actorID(r), id, body.Name, body.Category, body.Description, body.DestinationURL, body.Status)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, offerCardResponseFrom(card))
}

// AdminOfferTerms handles POST /admin/offers/{id}/terms.
func (s *Server) AdminOfferTerms(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		RateBps int `json:"rate_bps"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	version, effective, err := s.Offers.AddTerms(r.Context(), actorID(r), id, body.RateBps)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		Version       int    `json:"version"`
		RateBps       int    `json:"rate_bps"`
		EffectiveFrom string `json:"effective_from"`
	}{Version: version, RateBps: body.RateBps, EffectiveFrom: effective.UTC().Format(time.RFC3339)})
}

// AdminIntegrationKeysList handles GET /admin/integration-keys.
func (s *Server) AdminIntegrationKeysList(w http.ResponseWriter, r *http.Request, params gen.AdminIntegrationKeysListParams) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		respond(w, http.StatusBadRequest, errBody("project_id required"))
		return
	}
	keys, err := s.Offers.ListKeys(r.Context(), projectID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		Items []offers.KeyCard `json:"items"`
	}{Items: keys})
}

// AdminIntegrationKeyCreate handles POST /admin/integration-keys.
func (s *Server) AdminIntegrationKeyCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID string `json:"project_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	pair, err := s.Offers.CreateKey(r.Context(), actorID(r), body.ProjectID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, struct {
		KeyId  string `json:"key_id"`
		Secret string `json:"secret"`
	}{KeyId: pair.KeyID, Secret: pair.Secret})
}

// AdminIntegrationKeyRotate handles POST /admin/integration-keys/{keyId}/rotate.
func (s *Server) AdminIntegrationKeyRotate(w http.ResponseWriter, r *http.Request, keyId string) {
	pair, err := s.Offers.RotateKey(r.Context(), actorID(r), keyId)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, struct {
		KeyId  string `json:"key_id"`
		Secret string `json:"secret"`
	}{KeyId: pair.KeyID, Secret: pair.Secret})
}

// AdminIntegrationKeyDeactivate handles POST /admin/integration-keys/{keyId}/deactivate.
func (s *Server) AdminIntegrationKeyDeactivate(w http.ResponseWriter, r *http.Request, keyId string) {
	if err := s.Offers.DeactivateKey(r.Context(), actorID(r), keyId); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdminWithdrawalsList handles GET /admin/withdrawals.
func (s *Server) AdminWithdrawalsList(w http.ResponseWriter, r *http.Request, params gen.AdminWithdrawalsListParams) {
	status := r.URL.Query().Get("status")
	partnerID := r.URL.Query().Get("partner_id")
	limit, offset := pagination(r)
	rows, total, err := s.Payouts.ListAdmin(r.Context(), status, partnerID, limit, offset)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type adminWithdrawal struct {
		withdrawalResponse
		PartnerName  string `json:"partner_name"`
		PartnerEmail string `json:"partner_email"`
	}
	out := make([]adminWithdrawal, 0, len(rows))
	for _, row := range rows {
		base := withdrawalResponseFrom(repository.WithdrawalRequest{
			ID: row.ID, PartnerID: row.PartnerID, AmountKopecks: row.AmountKopecks,
			Method: row.Method, Requisites: row.Requisites, Bank: row.Bank,
			FeeKopecks: row.FeeKopecks, UsdtAmount: row.UsdtAmount, Rate: row.Rate,
			Status: row.Status, Comment: row.Comment, DecidedAt: row.DecidedAt,
			PaidAt: row.PaidAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
		out = append(out, adminWithdrawal{withdrawalResponse: base, PartnerName: row.PartnerName, PartnerEmail: row.PartnerEmail})
	}
	respond(w, http.StatusOK, struct {
		Total int64             `json:"total"`
		Items []adminWithdrawal `json:"items"`
	}{Total: total, Items: out})
}

// AdminWithdrawalDecide handles POST /admin/withdrawals/{id}/decide.
func (s *Server) AdminWithdrawalDecide(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Decision string  `json:"decision"`
		Comment  *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	comment := ""
	if body.Comment != nil {
		comment = *body.Comment
	}
	row, err := s.Payouts.Decide(r.Context(), actorID(r), id, body.Decision, comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, withdrawalResponseFrom(row))
}

// AdminWithdrawalPay handles POST /admin/withdrawals/{id}/pay.
func (s *Server) AdminWithdrawalPay(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		ExternalTxID *string `json:"external_tx_id"`
		Comment      *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	tx := ""
	comment := ""
	if body.ExternalTxID != nil {
		tx = *body.ExternalTxID
	}
	if body.Comment != nil {
		comment = *body.Comment
	}
	row, err := s.Payouts.Pay(r.Context(), actorID(r), id, tx, comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, withdrawalResponseFrom(row))
}

// AdminFinanceRulesGet handles GET /admin/finance/rules.
func (s *Server) AdminFinanceRulesGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Payouts.GetConfig(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, payoutRulesFromConfig(cfg))
}

// AdminFinanceRulesPut handles PUT /admin/finance/rules.
func (s *Server) AdminFinanceRulesPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MinWithdrawKopecks *int64   `json:"min_withdraw_kopecks"`
		UsdtRate           *float64 `json:"usdt_rate"`
		SbpFeeFlatKopecks  *int64   `json:"sbp_fee_flat_kopecks"`
		SbpFeePercentBps   *int     `json:"sbp_fee_percent_bps"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	// Partial updates: read current, apply patches, write all.
	cur, err := s.Payouts.GetConfig(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	min := cur.MinWithdrawKopecks
	rate := cur.UsdtRate
	flat := cur.SbpFeeFlatKopecks
	pct := cur.SbpFeePercentBps
	if body.MinWithdrawKopecks != nil {
		min = *body.MinWithdrawKopecks
	}
	if body.UsdtRate != nil {
		rate = *body.UsdtRate
	}
	if body.SbpFeeFlatKopecks != nil {
		flat = *body.SbpFeeFlatKopecks
	}
	if body.SbpFeePercentBps != nil {
		pct = *body.SbpFeePercentBps
	}
	cfg, err := s.Payouts.UpdateRules(r.Context(), actorID(r), min, rate, flat, pct)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, payoutRulesFromConfig(cfg))
}

// AdminFinanceLedger handles GET /admin/finance/ledger.
func (s *Server) AdminFinanceLedger(w http.ResponseWriter, r *http.Request, params gen.AdminFinanceLedgerParams) {
	partnerID := r.URL.Query().Get("partner_id")
	from := parseTimeParam(r.URL.Query().Get("from"), time.Time{})
	to := parseTimeParam(r.URL.Query().Get("to"), time.Time{})
	limit, offset := pagination(r)
	q := r.Context()
	rows, err := s.Q.ListLedgerAdmin(q, repository.ListLedgerAdminParams{
		Column1: partnerID,
		Column2: repository.TimePtr(nilTime(from)),
		Column3: repository.TimePtr(nilTime(to)),
		Limit:   int32(limit), Offset: int32(offset),
	})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	total, err := s.Q.CountLedgerAdmin(q, repository.CountLedgerAdminParams{
		Column1: partnerID,
		Column2: repository.TimePtr(nilTime(from)),
		Column3: repository.TimePtr(nilTime(to)),
	})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type adminLedger struct {
		ledgerEntryResponse
		PartnerID   string  `json:"partner_id"`
		PartnerName string  `json:"partner_name"`
		Comment     *string `json:"comment"`
	}
	out := make([]adminLedger, 0, len(rows))
	for _, e := range rows {
		out = append(out, adminLedger{
			ledgerEntryResponse: ledgerEntryResponse{
				ID: strconv.FormatInt(e.ID, 10), Type: e.Type,
				AmountKopecks: e.AmountKopecks, BalanceAfterKopecks: e.BalanceAfterKopecks,
				CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
			},
			PartnerID: e.PartnerID, PartnerName: e.PartnerName, Comment: repository.TextToPtr(e.Comment),
		})
	}
	respond(w, http.StatusOK, struct {
		Total int64         `json:"total"`
		Items []adminLedger `json:"items"`
	}{Total: total, Items: out})
}

// AdminFinanceEarnings handles GET /admin/finance/earnings.
func (s *Server) AdminFinanceEarnings(w http.ResponseWriter, r *http.Request, params gen.AdminFinanceEarningsParams) {
	partnerID := r.URL.Query().Get("partner_id")
	offerID := r.URL.Query().Get("offer_id")
	from := parseTimeParam(r.URL.Query().Get("from"), time.Time{})
	to := parseTimeParam(r.URL.Query().Get("to"), time.Time{})
	limit, offset := pagination(r)
	ctx := r.Context()
	rows, err := s.Q.ListEarningsAdmin(ctx, repository.ListEarningsAdminParams{
		Column1: partnerID, Column2: offerID,
		Column3: repository.TimePtr(nilTime(from)), Column4: repository.TimePtr(nilTime(to)),
		Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	total, err := s.Q.CountEarningsAdmin(ctx, repository.CountEarningsAdminParams{
		Column1: partnerID, Column2: offerID,
		Column3: repository.TimePtr(nilTime(from)), Column4: repository.TimePtr(nilTime(to)),
	})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type adminEarning struct {
		ID                string `json:"id"`
		PartnerID         string `json:"partner_id"`
		OfferID           string `json:"offer_id"`
		ConversionEventID int64  `json:"conversion_event_id"`
		RateBps           int    `json:"rate_bps"`
		AmountKopecks     int64  `json:"amount_kopecks"`
		ExternalUserID    string `json:"external_user_id"`
		Reversed          bool   `json:"reversed"`
		CreatedAt         string `json:"created_at"`
	}
	out := make([]adminEarning, 0, len(rows))
	for _, e := range rows {
		out = append(out, adminEarning{
			ID: e.ID, PartnerID: e.PartnerID, OfferID: e.OfferID,
			ConversionEventID: e.ConversionEventID, RateBps: int(e.RateBps),
			AmountKopecks: e.AmountKopecks, ExternalUserID: e.ExternalUserID,
			Reversed: e.ReversedAt.Valid,
			CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	respond(w, http.StatusOK, struct {
		Total int64          `json:"total"`
		Items []adminEarning `json:"items"`
	}{Total: total, Items: out})
}

// AdminAnnouncementsList handles GET /admin/announcements.
func (s *Server) AdminAnnouncementsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.Admin.ListAnnouncements(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	out := make([]announcementResponse, 0, len(items))
	for _, a := range items {
		out = append(out, announcementResponse{
			ID: a.ID, Title: a.Title, Body: a.Body, Audience: a.Audience,
			IsPublished: a.IsPublished, PublishedAt: a.PublishedAt, DeletedAt: a.DeletedAt,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		})
	}
	respond(w, http.StatusOK, struct {
		Items []announcementResponse `json:"items"`
	}{Items: out})
}

type announcementResponse struct {
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

// AdminAnnouncementCreate handles POST /admin/announcements.
func (s *Server) AdminAnnouncementCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title      string   `json:"title"`
		Body       string   `json:"body"`
		Audience   string   `json:"audience"`
		PartnerIds []string `json:"partner_ids"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	if body.Audience == "" {
		body.Audience = "all"
	}
	ann, err := s.Admin.CreateAnnouncement(r.Context(), actorID(r), body.Title, body.Body, body.Audience, body.PartnerIds, true)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, ann)
}

// AdminAnnouncementUpdate handles PATCH /admin/announcements/{id}.
func (s *Server) AdminAnnouncementUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Title      *string  `json:"title"`
		Body       *string  `json:"body"`
		Audience   *string  `json:"audience"`
		PartnerIds []string `json:"partner_ids"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	ann, err := s.Admin.UpdateAnnouncement(r.Context(), actorID(r), id, body.Title, body.Body, body.Audience, body.PartnerIds, nil)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, ann)
}

// AdminAnnouncementDelete handles DELETE /admin/announcements/{id}.
func (s *Server) AdminAnnouncementDelete(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.Admin.DeleteAnnouncement(r.Context(), actorID(r), id); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdminBrandingGet handles GET /admin/platform/branding.
func (s *Server) AdminBrandingGet(w http.ResponseWriter, r *http.Request) {
	b, err := s.Admin.GetBranding(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, b)
}

// AdminBrandingPut handles PUT /admin/platform/branding.
func (s *Server) AdminBrandingPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          *string `json:"name"`
		TelegramURL   *string `json:"telegram_url"`
		AvatarMediaID *string `json:"avatar_media_id"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	name := "CashX"
	if body.Name != nil {
		name = *body.Name
	}
	b, err := s.Admin.UpdateBranding(r.Context(), actorID(r), name, body.TelegramURL, body.AvatarMediaID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, b)
}

// AdminMediaUpload handles POST /admin/media (outside OpenAPI; multipart).
func (s *Server) AdminMediaUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respond(w, http.StatusBadRequest, errBody("invalid_upload"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond(w, http.StatusBadRequest, errBody("file required"))
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key, err := s.Media.Upload(r.Context(), file, header.Size, contentType)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	mediaID, url, err := s.Admin.InsertMedia(r.Context(), actorID(r), s.Cfg.S3Bucket, key, contentType, header.Size)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, struct {
		MediaId string `json:"media_id"`
		Url     string `json:"url"`
	}{MediaId: mediaID, Url: url})
}

// AdminAuditList handles GET /admin/audit.
func (s *Server) AdminAuditList(w http.ResponseWriter, r *http.Request, params gen.AdminAuditListParams) {
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	limit, offset := pagination(r)
	ctx := r.Context()
	rows, err := s.Q.ListAuditLog(ctx, repository.ListAuditLogParams{
		Column1: entityType, Column2: entityID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	total, err := s.Q.CountAuditLog(ctx, repository.CountAuditLogParams{Column1: entityType, Column2: entityID})
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	type auditEntry struct {
		ID         string          `json:"id"`
		ActorEmail *string         `json:"actor_email"`
		Action     string          `json:"action"`
		EntityType string          `json:"entity_type"`
		EntityID   string          `json:"entity_id"`
		Changes    json.RawMessage `json:"changes"`
		CreatedAt  string          `json:"created_at"`
	}
	out := make([]auditEntry, 0, len(rows))
	for _, e := range rows {
		out = append(out, auditEntry{
			ID: strconv.FormatInt(e.ID, 10), ActorEmail: repository.TextToPtr(e.ActorEmail), Action: e.Action,
			EntityType: e.EntityType, EntityID: e.EntityID, Changes: e.Changes,
			CreatedAt: e.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	respond(w, http.StatusOK, struct {
		Total int64        `json:"total"`
		Items []auditEntry `json:"items"`
	}{Total: total, Items: out})
}

func nilTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
