package integrations

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"

	"cashx/internal/offers"
	"cashx/internal/platform/httpjson"
	"cashx/internal/repository"
)

type ctxKey int

const (
	projectIDKey ctxKey = iota
)

// Middleware authenticates a signed integration request by its key.
type Middleware struct {
	Q      *repository.Queries
	Offers *offers.Service
}

// Verify wraps a handler with HMAC authentication for X-CashX-* headers.
func (m *Middleware) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyID := r.Header.Get("X-CashX-Key")
		ts := r.Header.Get("X-CashX-Timestamp")
		sig := r.Header.Get("X-CashX-Signature")
		if keyID == "" || ts == "" || sig == "" {
			httpjson.Error(w, http.StatusUnauthorized, "invalid_signature")
			return
		}
		key, err := m.Q.GetIntegrationKeyByKeyID(r.Context(), keyID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				httpjson.Error(w, http.StatusUnauthorized, "invalid_key")
				return
			}
			httpjson.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !key.IsActive {
			httpjson.Error(w, http.StatusForbidden, "key_inactive")
			return
		}
		secret, err := m.Offers.DecryptSecret(key.SecretCiphertext)
		if err != nil {
			httpjson.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		body, err := readBody(r)
		if err != nil {
			httpjson.Error(w, http.StatusBadRequest, "invalid_payload")
			return
		}
		if err := verify(secret, ts, sig, string(body)); err != nil {
			httpjson.Error(w, http.StatusUnauthorized, "invalid_signature")
			return
		}
		// Restore the raw body so the handler can decode it exactly as signed.
		r.Body = io.NopCloser(bytes.NewReader(body))
		ctx := context.WithValue(r.Context(), projectIDKey, key.ProjectID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ProjectID returns the authenticated project id from the context.
func ProjectID(ctx context.Context) string {
	v, _ := ctx.Value(projectIDKey).(string)
	return v
}

// RateLimitKey extracts the key_id for integration rate limiting.
func RateLimitKey(r *http.Request) string {
	return r.Header.Get("X-CashX-Key")
}
