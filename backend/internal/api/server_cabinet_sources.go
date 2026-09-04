package api

import (
	"net/http"
	"time"

	"cashx/internal/api/gen"
	"cashx/internal/offers"
)

func parseRFC3339(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func sourceTotalsResponse(t offers.SourceTotals) gen.SourceTotals {
	return gen.SourceTotals{
		Clicks:        &t.Clicks,
		UniqueClicks:  &t.UniqueClicks,
		Registrations: &t.Registrations,
		FirstPayments: &t.FirstPayments,
		IncomeKopecks: &t.IncomeKopecks,
	}
}

func sourceResponse(s offers.Source) gen.Source {
	id := s.ID
	code := s.Code
	name := s.Name
	url := s.URL
	isDefault := s.IsDefault
	isActive := s.IsActive
	totals := sourceTotalsResponse(s.Totals)
	totals30d := sourceTotalsResponse(s.Totals30d)
	srcType := gen.SourceType(s.Type)
	if s.Type == "" {
		srcType = gen.SourceTypeLink
	}
	resp := gen.Source{
		Id: &id, Code: &code, Name: &name, Comment: s.Comment,
		GroupId: s.GroupID, GroupName: s.GroupName,
		IsDefault: &isDefault, IsActive: &isActive, Url: &url,
		Type: &srcType,
		Totals: &totals, Totals30d: &totals30d,
		CreatedAt: parseRFC3339(s.CreatedAt),
	}
	if s.RegistrationBonus != nil {
		resp.RegistrationBonus = s.RegistrationBonus
	}
	if s.Domain != nil {
		resp.Domain = s.Domain
	}
	if s.RedirectID != nil {
		resp.RedirectId = s.RedirectID
	}
	return resp
}

func sourceGroupResponse(g offers.Group) gen.SourceGroup {
	id := g.ID
	name := g.Name
	return gen.SourceGroup{
		Id: &id, Name: &name, Comment: g.Comment,
		CreatedAt: parseRFC3339(g.CreatedAt),
		UpdatedAt: parseRFC3339(g.UpdatedAt),
	}
}

// CabinetOfferSourcesList handles GET /cabinet/offers/{offerId}/sources.
func (s *Server) CabinetOfferSourcesList(w http.ResponseWriter, r *http.Request, offerId string) {
	partnerID := partnerIDFrom(r)
	items, err := s.Offers.ListSources(r.Context(), partnerID, offerId)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	out := struct {
		Items []gen.Source `json:"items"`
	}{Items: make([]gen.Source, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, sourceResponse(it))
	}
	respond(w, http.StatusOK, out)
}

// CabinetOfferSourceCreate handles POST /cabinet/offers/{offerId}/sources.
func (s *Server) CabinetOfferSourceCreate(w http.ResponseWriter, r *http.Request, offerId string) {
	var body gen.SourceInput
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	partnerID := partnerIDFrom(r)
	isDefault := body.IsDefault != nil && *body.IsDefault
	// Determine type
	typ := "link"
	if body.Type != nil && *body.Type == gen.SourceInputTypePromo {
		typ = "promo"
	}
	var src offers.Source
	var err error
	if typ == "promo" {
		src, err = s.Offers.CreatePromoSource(r.Context(), partnerID, offerId, body.Name, body.Code, body.RegistrationBonus, body.Comment, body.GroupId)
	} else {
		// link with optional domain/redirect
		if body.Domain != nil || body.RedirectId != nil {
			src, err = s.Offers.CreateLinkSource(r.Context(), partnerID, offerId, body.Name, body.Code, body.Comment, body.GroupId, body.Domain, body.RedirectId, isDefault)
		} else {
			src, err = s.Offers.CreateSource(r.Context(), partnerID, offerId, body.Name, body.Code, body.Comment, body.GroupId, isDefault)
		}
	}
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, sourceResponse(src))
}

// CabinetOfferSourceUpdate handles PATCH /cabinet/offers/{offerId}/sources/{sourceId}.
func (s *Server) CabinetOfferSourceUpdate(w http.ResponseWriter, r *http.Request, offerId, sourceId string) {
	var body gen.SourceUpdate
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	partnerID := partnerIDFrom(r)
	isActive := body.IsActive == nil || *body.IsActive
	isDefault := body.IsDefault != nil && *body.IsDefault
	// If promo-specific fields present, try extended update
	if body.Type != nil || body.RegistrationBonus != nil || body.Domain != nil || body.RedirectId != nil {
		// For now, handle code/name/comment/group/isActive/isDefault via existing UpdateSource
		// and attempt to update extended fields via raw query if needed
		src, err := s.Offers.UpdateSource(r.Context(), partnerID, offerId, sourceId, body.Name, body.Code, body.Comment, body.GroupId, isActive, isDefault)
		if err != nil {
			writeErr(s.Log, w, err)
			return
		}
		// Validate the domain against the offer's active domains (the raw update
		// below would otherwise store an arbitrary value).
		if body.Domain != nil && *body.Domain != "" {
			norm, derr := s.Offers.ValidateSourceDomain(r.Context(), offerId, *body.Domain)
			if derr != nil {
				writeErr(s.Log, w, derr)
				return
			}
			body.Domain = norm
		}
		// Best-effort: update extended fields directly if provided
		if body.RegistrationBonus != nil {
			_, _ = s.Pool.Exec(r.Context(), `UPDATE tracking_links SET registration_bonus=$1 WHERE id=$2`, *body.RegistrationBonus, sourceId)
		}
		if body.Domain != nil {
			_, _ = s.Pool.Exec(r.Context(), `UPDATE tracking_links SET domain=$1 WHERE id=$2`, *body.Domain, sourceId)
		}
		if body.RedirectId != nil {
			_, _ = s.Pool.Exec(r.Context(), `UPDATE tracking_links SET redirect_id=$1::uuid WHERE id=$2`, *body.RedirectId, sourceId)
		}
		// Re-fetch
		if refreshed, err := s.Offers.ListSources(r.Context(), partnerID, offerId); err == nil {
			for _, it := range refreshed {
				if it.ID == sourceId {
					src = it
					break
				}
			}
		}
		respond(w, http.StatusOK, sourceResponse(src))
		return
	}
	src, err := s.Offers.UpdateSource(r.Context(), partnerID, offerId, sourceId, body.Name, body.Code, body.Comment, body.GroupId, isActive, isDefault)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, sourceResponse(src))
}

// CabinetOfferSourceDelete handles DELETE /cabinet/offers/{offerId}/sources/{sourceId}.
func (s *Server) CabinetOfferSourceDelete(w http.ResponseWriter, r *http.Request, offerId, sourceId string) {
	partnerID := partnerIDFrom(r)
	if err := s.Offers.DeleteSource(r.Context(), partnerID, offerId, sourceId); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CabinetSourceGroupsList handles GET /cabinet/source-groups.
func (s *Server) CabinetSourceGroupsList(w http.ResponseWriter, r *http.Request) {
	partnerID := partnerIDFrom(r)
	items, err := s.Offers.ListGroups(r.Context(), partnerID)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	out := struct {
		Items []gen.SourceGroup `json:"items"`
	}{Items: make([]gen.SourceGroup, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, sourceGroupResponse(it))
	}
	respond(w, http.StatusOK, out)
}

// CabinetSourceGroupCreate handles POST /cabinet/source-groups.
func (s *Server) CabinetSourceGroupCreate(w http.ResponseWriter, r *http.Request) {
	var body gen.SourceGroupInput
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	partnerID := partnerIDFrom(r)
	g, err := s.Offers.CreateGroup(r.Context(), partnerID, body.Name, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusCreated, sourceGroupResponse(g))
}

// CabinetSourceGroupUpdate handles PATCH /cabinet/source-groups/{groupId}.
func (s *Server) CabinetSourceGroupUpdate(w http.ResponseWriter, r *http.Request, groupId string) {
	var body gen.SourceGroupInput
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	partnerID := partnerIDFrom(r)
	g, err := s.Offers.UpdateGroup(r.Context(), partnerID, groupId, body.Name, body.Comment)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, sourceGroupResponse(g))
}

// CabinetSourceGroupDelete handles DELETE /cabinet/source-groups/{groupId}.
func (s *Server) CabinetSourceGroupDelete(w http.ResponseWriter, r *http.Request, groupId string) {
	partnerID := partnerIDFrom(r)
	if err := s.Offers.DeleteGroup(r.Context(), partnerID, groupId); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
