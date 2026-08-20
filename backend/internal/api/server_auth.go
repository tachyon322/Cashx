package api

import (
	"net/http"

	"cashx/internal/api/gen"
	"cashx/internal/auth"
)

// userMe builds the UserMe API shape from the principal.
func userMe(u *auth.User) gen.UserMe {
	role := gen.UserMeRole(u.Role)
	out := gen.UserMe{
		Id:         &u.ID,
		Email:      &u.Email,
		Name:       &u.Name,
		Role:       &role,
		IsApproved: new(u.Partner != nil && u.Partner.IsApproved),
		IsBlocked:  new(u.Partner != nil && u.Partner.IsBlocked),
	}
	if u.Partner != nil {
		out.Partner = &struct {
			ReferralCode   *string `json:"referral_code,omitempty"`
			TelegramUserId *int64  `json:"telegram_user_id,omitempty"`
		}{ReferralCode: &u.Partner.ReferralCode, TelegramUserId: u.Partner.TelegramUserID}
	}
	if len(u.Staff) > 0 {
		roles := make([]string, 0, len(u.Staff))
		for _, r := range u.Staff {
			roles = append(roles, r.Role)
		}
		out.Staff = &struct {
			Roles *[]string `json:"roles,omitempty"`
		}{Roles: &roles}
	}
	return out
}

// AuthRegister handles POST /auth/register.
func (s *Server) AuthRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string  `json:"name"`
		Email        string  `json:"email"`
		Password     string  `json:"password"`
		ReferralCode *string `json:"referral_code"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	ref := ""
	if body.ReferralCode != nil {
		ref = *body.ReferralCode
	}
	user, err := s.AuthService.Register(r.Context(), body.Name, body.Email, body.Password, ref)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	me := gen.UserMe{
		Id:         &user.ID,
		Email:      &user.Email,
		Name:       &user.Name,
		Role:       new(gen.UserMeRole("partner")),
		IsApproved: new(false),
		IsBlocked:  new(false),
	}
	profile, err := s.Q.GetPartnerProfileByUserID(r.Context(), user.ID)
	if err == nil {
		me.Partner = &struct {
			ReferralCode   *string `json:"referral_code,omitempty"`
			TelegramUserId *int64  `json:"telegram_user_id,omitempty"`
		}{ReferralCode: &profile.ReferralCode}
	}
	respond(w, http.StatusCreated, struct {
		User gen.UserMe `json:"user"`
	}{User: me})
}

// AuthMe handles GET /auth/me (requires a session; see router).
func (s *Server) AuthMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		respond(w, http.StatusUnauthorized, errBody("unauthorized"))
		return
	}
	respond(w, http.StatusOK, struct {
		User gen.UserMe `json:"user"`
	}{User: userMe(u)})
}

// AuthPasswordResetRequest handles POST /auth/password-reset/request.
func (s *Server) AuthPasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	token, err := s.AuthService.PasswordResetRequest(r.Context(), body.Email)
	if err != nil {
		writeErr(s.Log, w, err)
		return
	}
	resp := struct {
		ResetToken *string `json:"reset_token"`
	}{}
	if token != "" {
		resp.ResetToken = &token
	}
	respond(w, http.StatusAccepted, resp)
}

// AuthPasswordResetConfirm handles POST /auth/password-reset/confirm.
func (s *Server) AuthPasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	if err := s.AuthService.PasswordResetConfirm(r.Context(), body.Token, body.NewPassword); err != nil {
		writeErr(s.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
