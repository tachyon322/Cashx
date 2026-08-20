package platform

import (
	"net"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit returns a middleware that allows up to `limit` requests per 60s
// window per key. Key extraction is provided by the caller.
func RateLimit(rdb *redis.Client, scope string, limit int, key func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := key(r)
			if k == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			redisKey := "rl:" + scope + ":" + k
			// INCR + EXPIRE: a fresh key gets count 1; existing keys keep the
			// window from their first request.
			pipe := rdb.TxPipeline()
			incr := pipe.Incr(ctx, redisKey)
			pipe.Expire(ctx, redisKey, 60*time.Second)
			if _, err := pipe.Exec(ctx); err != nil {
				// Redis down: fail open rather than break the platform.
				next.ServeHTTP(w, r)
				return
			}
			if incr.Val() > int64(limit) {
				w.Header().Set("Retry-After", "60")
				httpjsonError(w, http.StatusTooManyRequests, "rate_limited")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func httpjsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"message":"` + message + `"}`))
}

// RemoteIPKey extracts the client IP for rate limiting.
func RemoteIPKey(r *http.Request) string {
	// X-Forwarded-For is trusted only behind a proxy that sets it; in dev it is absent.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// KeyFromHeader returns a rate limit key from a request header.
func KeyFromHeader(header string) func(*http.Request) string {
	return func(r *http.Request) string {
		return r.Header.Get(header)
	}
}

// IntParam reads an integer query parameter with a default.
func IntParam(r *http.Request, name string, def int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}
