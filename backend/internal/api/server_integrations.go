package api

import (
	"io"
	"net/http"

	"cashx/internal/integrations"
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
		Status string  `json:"status"`
		Reason *string `json:"reason"`
	}{Status: result.Status}
	if result.Reason != "" {
		resp.Reason = &result.Reason
	}
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
