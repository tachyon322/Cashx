package api

import "net/http"

// PublicBrandingGet handles GET /public/branding — платформенный брендинг
// (название, Telegram, аватар) без авторизации: используется кабинетом
// партнёра и публичными страницами (вход, splash).
func (s *Server) PublicBrandingGet(w http.ResponseWriter, r *http.Request) {
	b, err := s.Admin.GetBranding(r.Context())
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	respond(w, http.StatusOK, b)
}
