package tracking

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"cashx/internal/platform"
	"cashx/internal/platform/httpjson"
	"cashx/internal/platform/httpmw"
)

// RedirectDeps are the redirect service dependencies.
type RedirectDeps struct {
	Cfg   platform.Config
	Log   *slog.Logger
	Pool  *pgxpool.Pool
	Redis *redis.Client
	// ResolveHostOrigin maps the request host to an active offer domain
	// (origin as stored in offer_domains). Nil defaults to a lookup in Pool;
	// tests inject a fake.
	ResolveHostOrigin func(ctx context.Context, host string) (string, bool)
}

// NewRedirectHandler builds the redirect router.
func NewRedirectHandler(d RedirectDeps) http.Handler {
	r := httpmw.Chain(d.Log, d.Cfg.FrontendOrigin)
	rl := noopMiddleware
	if d.Cfg.RateLimit && d.Redis != nil {
		rl = platform.RateLimit(d.Redis, "redirect", 100, platform.RemoteIPKey)
	}
	resolveHost := d.ResolveHostOrigin
	if resolveHost == nil && d.Pool != nil {
		resolveHost = offerDomainResolver(d.Pool)
	}
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Group(func(r chi.Router) {
		r.Use(rl)
		handle := func(w http.ResponseWriter, req *http.Request) {
			// fallback is where a click goes when it cannot be resolved.
			// It must never be the CashX web (партнёрка): a click stranded
			// there is lost conversion, so production points it at the casino.
			// Resolved lazily — only error paths pay for the host lookup.
			fallback := func() string {
				return fallbackOrigin(req, d.Cfg, resolveHost)
			}
			code := normalizeCode(chi.URLParam(req, "code"))
			if code == "" {
				code = normalizeCode(req.PathValue("code"))
			}
			if code == "" || len(code) > 32 {
				http.Redirect(w, req, fallback(), http.StatusFound)
				return
			}
			result, err := RecordClick(req.Context(), d.Pool, code, platform.RemoteIPKey(req), req.UserAgent(), req.Referer())
			if err != nil {
				if errors.Is(err, platform.ErrNotFound) {
					http.Redirect(w, req, fallback(), http.StatusFound)
					return
				}
				d.Log.Error("record click", "err", err)
				http.Redirect(w, req, fallback(), http.StatusFound)
				return
			}
			token, err := SignClickToken(d.Cfg.ClickTokenSecret, d.Cfg.ClickTokenTTL, result.ClickID)
			if err != nil {
				d.Log.Error("sign click token", "err", err)
				http.Redirect(w, req, fallback(), http.StatusFound)
				return
			}
			dest, err := url.Parse(result.Destination)
			if err == nil {
				q := dest.Query()
				q.Set("click_token", token)
				q.Set("ref", code)
				dest.RawQuery = q.Encode()
				http.Redirect(w, req, dest.String(), http.StatusFound)
				return
			}
			sep := "?"
			if strings.Contains(result.Destination, "?") {
				sep = "&"
			}
			http.Redirect(w, req, result.Destination+sep+"click_token="+token+"&ref="+code, http.StatusFound)
		}
		r.Get("/c/{code}", handle)
		r.Get("/r/{code}", handle)
	})
	return r
}

func noopMiddleware(h http.Handler) http.Handler { return h }

// normalizeCode cleans a tracking code coming from the URL path before the DB
// lookup. Messengers and social networks (VK, Telegram) often keep a trailing
// space glued to the link, so "LITGWIN%20" arrives as "LITGWIN " and would
// otherwise miss the link and fall back to the CashX landing page.
func normalizeCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

// fallbackOrigin picks where an unresolved click lands. A click that arrived
// on a casino domain (Caddy proxies {domain}/r/* straight here and keeps the
// Host) goes back to that same casino: with several projects on one CashX a
// single global fallback would send a broken StarX link to Litgame. Anything
// else — the CashX host itself (/c/ links), an unknown host, a failed lookup —
// uses RedirectFallback, then FrontendOrigin.
func fallbackOrigin(req *http.Request, cfg platform.Config, resolveHost func(context.Context, string) (string, bool)) string {
	if resolveHost != nil {
		if origin, ok := resolveHost(req.Context(), req.Host); ok && origin != "" {
			return origin
		}
	}
	if f := strings.TrimSpace(cfg.RedirectFallback); f != "" {
		return f
	}
	return cfg.FrontendOrigin
}

// hostOriginCandidates turns a request Host into the origins it may be stored
// as in offer_domains (lowercased scheme://host, see normalizeDomainURL):
// port and trailing dot dropped, with and without "www.".
func hostOriginCandidates(host string) []string {
	h := strings.TrimSpace(host)
	if hp, _, err := net.SplitHostPort(h); err == nil {
		h = hp
	}
	h = strings.TrimSuffix(strings.ToLower(h), ".")
	if h == "" {
		return nil
	}
	hosts := []string{h}
	if bare, ok := strings.CutPrefix(h, "www."); ok {
		if bare != "" {
			hosts = append(hosts, bare)
		}
	} else {
		hosts = append(hosts, "www."+h)
	}
	out := make([]string, 0, len(hosts)*2)
	for _, x := range hosts {
		out = append(out, "https://"+x, "http://"+x)
	}
	return out
}

// offerDomainResolver looks the request host up among active offer domains.
// It runs only on the fallback path — possibly right after a DB failure — so
// it is bounded by a short timeout and any error means "not a casino host".
func offerDomainResolver(pool *pgxpool.Pool) func(context.Context, string) (string, bool) {
	return func(ctx context.Context, host string) (string, bool) {
		cands := hostOriginCandidates(host)
		if len(cands) == 0 {
			return "", false
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		var origin string
		err := pool.QueryRow(ctx,
			`SELECT url FROM offer_domains WHERE is_active AND url = ANY($1) ORDER BY is_main DESC LIMIT 1`,
			cands).Scan(&origin)
		if err != nil {
			return "", false
		}
		return origin, true
	}
}
