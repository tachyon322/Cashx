package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"cashx/internal/integrations"
	"cashx/internal/repository"
)

// IntegrationsEvent handles POST /integrations/events.
func (s *Server) IntegrationsEvent(w http.ResponseWriter, r *http.Request) {
	projectID := integrations.ProjectID(r.Context())
	if projectID == "" {
		respond(w, http.StatusUnauthorized, errBody("invalid_key"))
		return
	}
	body := readRawBody(r)
	result, err := s.Integrations.Process(r.Context(), projectID, body)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		Status string              `json:"status"`
		Reason *string             `json:"reason"`
		Source *integrations.EventSource `json:"source,omitempty"`
	}{Status: result.Status}
	if result.Reason != "" {
		resp.Reason = &result.Reason
	}
	resp.Source = result.Source
	respond(w, http.StatusAccepted, resp)
}

// readRawBody returns the raw request body stored by the HMAC middleware.
func readRawBody(r *http.Request) []byte {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	return body
}

// IntegrationsSource handles GET /integrations/source?code=CODE (HMAC-signed).
// Lets an integrated project resolve a source/promo code — type, activity,
// registration bonus — inside its own project scope, so the project does not
// need a local copy of the sources table to grant registration bonuses.
func (s *Server) IntegrationsSource(w http.ResponseWriter, r *http.Request) {
	projectID := integrations.ProjectID(r.Context())
	if projectID == "" {
		respond(w, http.StatusUnauthorized, errBody("invalid_key"))
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code")))
	if code == "" || len(code) > 32 {
		respond(w, http.StatusBadRequest, errBody("invalid_payload"))
		return
	}
	link, err := s.Q.GetTrackingLinkByCodeForProject(r.Context(), repository.GetTrackingLinkByCodeForProjectParams{
		Code: code, ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond(w, http.StatusNotFound, errBody("source_not_found"))
			return
		}
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		Code             string `json:"code"`
		Type             string `json:"type"`
		IsPromo          bool   `json:"is_promo"`
		IsActive         bool   `json:"is_active"`
		AccessActive     bool   `json:"access_active"`
		RegistrationBonus *int32 `json:"registration_bonus,omitempty"`
	}{Code: link.Code, Type: link.Type, IsActive: link.IsActive}
	resp.IsPromo = resp.Type == "promo"
	resp.AccessActive = link.AccessStatus == "active"
	if link.RegistrationBonus.Valid {
		resp.RegistrationBonus = &link.RegistrationBonus.Int32
	}
	respond(w, http.StatusOK, resp)
}
