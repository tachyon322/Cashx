package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"cashx/internal/offers"
)

// Admin offer-domain handlers: the offer catalog manages the domain list
// (one main + backup mirrors) used by partner source links.

// AdminOfferDomainsList handles GET /admin/offers/{offerId}/domains.
func (s *Server) AdminOfferDomainsList(w http.ResponseWriter, r *http.Request) {
	offerID := chi.URLParam(r, "offerId")
	items, err := s.Offers.ListOfferDomains(r.Context(), offerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	if items == nil {
		items = []offers.OfferDomain{}
	}
	respond(w, http.StatusOK, map[string]interface{}{"items": items})
}

// AdminOfferDomainCreate handles POST /admin/offers/{offerId}/domains.
func (s *Server) AdminOfferDomainCreate(w http.ResponseWriter, r *http.Request) {
	offerID := chi.URLParam(r, "offerId")
	var body struct {
		URL      string  `json:"url"`
		IsMain   *bool   `json:"is_main"`
		IsActive *bool   `json:"is_active"`
		Comment  *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	isMain := body.IsMain != nil && *body.IsMain
	row, err := s.Offers.CreateOfferDomain(r.Context(), offerID, body.URL, isMain, body.IsActive, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]interface{}{"domain": row})
}

// AdminOfferDomainUpdate handles PATCH /admin/offers/{offerId}/domains/{id}.
func (s *Server) AdminOfferDomainUpdate(w http.ResponseWriter, r *http.Request) {
	offerID := chi.URLParam(r, "offerId")
	id := chi.URLParam(r, "id")
	var body struct {
		URL      *string `json:"url"`
		IsMain   *bool   `json:"is_main"`
		IsActive *bool   `json:"is_active"`
		Comment  *string `json:"comment"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	row, err := s.Offers.UpdateOfferDomain(r.Context(), offerID, id, body.URL, body.IsActive, body.IsMain, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{"domain": row})
}

// AdminOfferDomainDelete handles DELETE /admin/offers/{offerId}/domains/{id}.
func (s *Server) AdminOfferDomainDelete(w http.ResponseWriter, r *http.Request) {
	offerID := chi.URLParam(r, "offerId")
	id := chi.URLParam(r, "id")
	if err := s.Offers.DeleteOfferDomain(r.Context(), offerID, id); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CabinetOfferDomainsList handles GET /cabinet/offers/{offerId}/domains —
// active domains of a joined offer (main first) for the source link editor.
func (s *Server) CabinetOfferDomainsList(w http.ResponseWriter, r *http.Request) {
	offerID := chi.URLParam(r, "offerId")
	partnerID := partnerIDFrom(r)
	items, err := s.Offers.ListActiveOfferDomainsForPartner(r.Context(), partnerID, offerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	if items == nil {
		items = []offers.OfferDomain{}
	}
	respond(w, http.StatusOK, map[string]interface{}{"items": items})
}
