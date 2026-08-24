// Package api wires the HTTP server: dependencies, auth middlewares, handlers.
package api

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/admin"
	"cashx/internal/audit"
	"cashx/internal/auth"
	"cashx/internal/integrations"
	"cashx/internal/offers"
	"cashx/internal/partners"
	"cashx/internal/payouts"
	"cashx/internal/platform"
	"cashx/internal/platform/httpjson"
	"cashx/internal/projects"
	"cashx/internal/repository"
)

// Server implements the generated ServerInterface.
type Server struct {
	Cfg          platform.Config
	Log          *slog.Logger
	Pool         *pgxpool.Pool
	Q            *repository.Queries
	AuthService  *auth.Service
	Limen        *auth.Limen
	Middleware   *auth.Middlewares
	Audit        *audit.Recorder
	Projects     *projects.Service
	Offers       *offers.Service
	Integrations *integrations.Service
	Payouts      *payouts.Service
	Partners     *partners.Service
	Admin        *admin.Service
}

// New assembles the server dependencies.
func New(cfg platform.Config, log *slog.Logger, pool *pgxpool.Pool, limenAuth *auth.Limen) *Server {
	q := repository.New(pool)
	authService := &auth.Service{Pool: pool, Limen: limenAuth, Cfg: cfg}
	auditRec := &audit.Recorder{Q: q}
	return &Server{
		Cfg:         cfg,
		Log:         log,
		Pool:        pool,
		Q:           q,
		AuthService: authService,
		Limen:       limenAuth,
		Middleware:  &auth.Middlewares{Q: q, Limen: limenAuth},
		Audit:       auditRec,
		Projects:    &projects.Service{Pool: pool, Audit: auditRec},
		Offers:      &offers.Service{Pool: pool, Audit: auditRec, IntegrationEncryptionKey: cfg.IntegrationKeyEncryptionKey, WebOrigin: cfg.WebOrigin},
		Integrations: &integrations.Service{Pool: pool, ClickTokenSecret: cfg.ClickTokenSecret},
		Payouts:     &payouts.Service{Pool: pool, Audit: auditRec},
		Partners:    &partners.Service{Pool: pool, WebOrigin: cfg.WebOrigin},
		Admin:       &admin.Service{Pool: pool, Auth: authService, Limen: limenAuth, Audit: auditRec, WebOrigin: cfg.WebOrigin},
	}
}

// respond writes a JSON response.
func respond(w http.ResponseWriter, status int, v any) {
	httpjson.WriteJSON(w, status, v)
}

// writeErr maps a domain error to an HTTP response.
func writeErr(log *slog.Logger, w http.ResponseWriter, err error) {
	status := platform.HTTPStatus(err)
	msg := platform.Message(err)
	if status == http.StatusInternalServerError {
		log.Error("internal error", "err", err)
		msg = "internal error"
	}
	httpjson.Error(w, status, msg)
}

// userFrom extracts the authenticated principal.
func userFrom(r *http.Request) *auth.User {
	return auth.UserFrom(r.Context())
}

// partnerIDFrom extracts the partner profile id of the authenticated partner.
func partnerIDFrom(r *http.Request) string {
	u := userFrom(r)
	if u == nil || u.Partner == nil {
		return ""
	}
	return u.Partner.PartnerID
}

// actorID returns the user id for audit records (nil for system actions).
func actorID(r *http.Request) *string {
	if u := userFrom(r); u != nil {
		return &u.ID
	}
	return nil
}
