package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"cashx/internal/platform/httpjson"
	"cashx/internal/repository"
)

type ctxKey int

const userCtxKey ctxKey = iota

// WithUser attaches the authenticated principal to the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom returns the principal from the context (nil when unauthenticated).
func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userCtxKey).(*User)
	return u
}

// Middlewares holds the dependencies of the auth middleware chain.
type Middlewares struct {
	Q     *repository.Queries
	Limen *Limen
}

// loadUser resolves the principal (users + partner/staff profile) for a
// Limen session user.
func (m *Middlewares) loadUser(ctx context.Context, id any) (*User, error) {
	q := m.Q
	user, err := q.GetUserByID(ctx, fmt.Sprint(id))
	if err != nil {
		return nil, err
	}
	u := &User{ID: user.ID, Email: user.Email, Name: user.Name, Role: user.Role, IsActive: user.IsActive}

	// A staff user can also have a partner identity. Load both profiles instead
	// of treating the two roles as mutually exclusive.
	p, err := q.GetPartnerProfileByUserID(ctx, user.ID)
	if err == nil {
		u.Partner = &PartnerInfo{
			PartnerID:      p.ID,
			IsApproved:     p.IsApproved,
			IsBlocked:      p.IsBlocked,
			ReferralCode:   p.ReferralCode,
			TelegramUserID: repository.Int64ToPtr(p.TelegramUserID),
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	roles, err := q.GetStaffRolesByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		u.Staff = append(u.Staff, StaffRole{Role: r.Role, ProjectID: repository.UUIDToPtr(r.ProjectID)})
	}
	return u, nil
}

// RequireUser resolves the Limen session cookie and attaches the user; 401 otherwise.
func (m *Middlewares) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vs, err := m.Limen.GetSession(r)
		if err != nil {
			httpjson.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		// Sliding expiration: reissue the refreshed cookie when Limen extended
		// the session during validation.
		if vs.Refreshed != nil && vs.Refreshed.Cookie != nil {
			http.SetCookie(w, vs.Refreshed.Cookie)
		}
		user, err := m.loadUser(r.Context(), vs.User.ID)
		if err != nil {
			httpjson.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

// RequirePartner requires an approved, unblocked partner session.
func (m *Middlewares) RequirePartner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFrom(r.Context())
		if user == nil || user.Partner == nil {
			httpjson.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if user.Role == "staff" && !user.IsActive {
			httpjson.Error(w, http.StatusForbidden, "account_blocked")
			return
		}
		if user.Partner.IsBlocked {
			httpjson.Error(w, http.StatusForbidden, "account_blocked")
			return
		}
		if !user.Partner.IsApproved {
			httpjson.Error(w, http.StatusForbidden, "pending_approval")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireStaff requires a staff session with one of the given roles.
// superadmin passes any check.
func (m *Middlewares) RequireStaff(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFrom(r.Context())
			if user == nil || user.Role != "staff" {
				httpjson.Error(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !user.IsActive {
				httpjson.Error(w, http.StatusForbidden, "account_blocked")
				return
			}
			allowed := false
			for _, sr := range user.Staff {
				if sr.Role == "superadmin" {
					allowed = true
					break
				}
				for _, role := range roles {
					if sr.Role == role {
						allowed = true
						break
					}
				}
				if allowed {
					break
				}
			}
			if !allowed {
				httpjson.Error(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
