package api

import (
	"net"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"cashx/internal/platform"
	"cashx/internal/platform/httpjson"
)

// decodeBody parses and validates a JSON request body.
func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.Join(platform.ErrValidation, err)
	}
	return nil
}

// clientIP extracts the client IP for audit/logging.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// errBody is the canonical error shape.
func errBody(msg string) httpjson.ErrorBody {
	return httpjson.ErrorBody{Message: msg}
}
