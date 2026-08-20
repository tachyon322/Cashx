package tracking

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"cashx/internal/platform"
	"cashx/internal/platform/httpjson"
	"cashx/internal/platform/httpmw"
	"cashx/internal/repository"
)

// RedirectDeps are the redirect service dependencies.
type RedirectDeps struct {
	Cfg             platform.Config
	Log             *slog.Logger
	Pool            *pgxpool.Pool
	Redis           *redis.Client
}

// NewRedirectHandler builds the redirect router.
func NewRedirectHandler(d RedirectDeps) http.Handler {
	r := httpmw.Chain(d.Log, d.Cfg.FrontendOrigin)
	rl := noopMiddleware
	if d.Cfg.RateLimit && d.Redis != nil {
		rl = platform.RateLimit(d.Redis, "redirect", 100, platform.RemoteIPKey)
	}
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Group(func(r chi.Router) {
		r.Use(rl)
		r.Get("/c/{code}", func(w http.ResponseWriter, req *http.Request) {
			code := strings.ToUpper(req.PathValue("code"))
			if code == "" || len(code) > 12 {
				http.Redirect(w, req, d.Cfg.FrontendOrigin, http.StatusFound)
				return
			}
			q := repository.New(d.Pool)
			result, err := RecordClick(req.Context(), q, code, platform.RemoteIPKey(req), req.UserAgent(), req.Referer())
			if err != nil {
				if errors.Is(err, platform.ErrNotFound) {
					http.Redirect(w, req, d.Cfg.FrontendOrigin, http.StatusFound)
					return
				}
				d.Log.Error("record click", "err", err)
				http.Redirect(w, req, d.Cfg.FrontendOrigin, http.StatusFound)
				return
			}
			token, err := SignClickToken(d.Cfg.ClickTokenSecret, d.Cfg.ClickTokenTTL, result.ClickID)
			if err != nil {
				d.Log.Error("sign click token", "err", err)
				http.Redirect(w, req, d.Cfg.FrontendOrigin, http.StatusFound)
				return
			}
			sep := "?"
			if strings.Contains(result.Destination, "?") {
				sep = "&"
			}
			http.Redirect(w, req, result.Destination+sep+"click_token="+token, http.StatusFound)
		})
	})
	return r
}

func noopMiddleware(h http.Handler) http.Handler { return h }
