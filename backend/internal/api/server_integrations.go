package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"cashx/internal/integrations"
	"cashx/internal/repository"
	"cashx/internal/tracking"
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

// IntegrationsSource handles GET /integrations/source?code=CODE or ?click_token=TOKEN (HMAC-signed).
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
	clickToken := strings.TrimSpace(r.URL.Query().Get("click_token"))
	if code == "" && clickToken == "" {
		respond(w, http.StatusBadRequest, errBody("invalid_payload"))
		return
	}
	if len(code) > 32 {
		respond(w, http.StatusBadRequest, errBody("invalid_payload"))
		return
	}

	type SourceInfoResp struct {
		Code              string `json:"code"`
		Type              string `json:"type"`
		IsPromo           bool   `json:"is_promo"`
		IsActive          bool   `json:"is_active"`
		AccessActive      bool   `json:"access_active"`
		RegistrationBonus *int32 `json:"registration_bonus,omitempty"`
	}

	if clickToken != "" {
		clickID, err := tracking.VerifyClickToken(s.Cfg.ClickTokenSecret, clickToken)
		if err == nil {
			var linkCode, linkType, accessStatus string
			var isActive bool
			var regBonus pgtype.Int4
			query := `SELECT tl.code, tl.type, tl.is_active, tl.registration_bonus, a.status AS access_status
FROM tracking_clicks c
JOIN tracking_links tl ON tl.id = c.tracking_link_id
JOIN partner_offer_accesses a ON a.id = tl.partner_offer_access_id
JOIN offers o ON o.id = a.offer_id
WHERE c.id = $1 AND o.project_id = $2
LIMIT 1`
			qErr := s.Pool.QueryRow(r.Context(), query, clickID, projectID).Scan(
				&linkCode, &linkType, &isActive, &regBonus, &accessStatus,
			)
			if qErr == nil {
				resp := SourceInfoResp{
					Code:         linkCode,
					Type:         linkType,
					IsPromo:      linkType == "promo",
					IsActive:     isActive,
					AccessActive: accessStatus == "active",
				}
				if regBonus.Valid {
					resp.RegistrationBonus = &regBonus.Int32
				}
				respond(w, http.StatusOK, resp)
				return
			} else if !errors.Is(qErr, pgx.ErrNoRows) {
				writeErr(s.Log, w, qErr)
				return
			}
		}
		if code == "" {
			respond(w, http.StatusNotFound, errBody("source_not_found"))
			return
		}
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
	resp := SourceInfoResp{
		Code:         link.Code,
		Type:         link.Type,
		IsPromo:      link.Type == "promo",
		IsActive:     link.IsActive,
		AccessActive: link.AccessStatus == "active",
	}
	if link.RegistrationBonus.Valid {
		resp.RegistrationBonus = &link.RegistrationBonus.Int32
	}
	respond(w, http.StatusOK, resp)
}
