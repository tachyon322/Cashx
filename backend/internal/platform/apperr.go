// Package platform holds shared platform helpers, including the canonical
// error taxonomy used by the CashX services.
package platform

import (
	"errors"
	"net/http"
	"strings"
)

// Sentinel error kinds. Domain errors wrap one of these with fmt.Errorf("%w: %s", kind, message).
var (
	ErrNotFound     = errors.New("not_found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation")
	ErrUnauthorized = errors.New("unauthorized")
)

// HTTPStatus maps an error to its HTTP status code.
func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// Message extracts the human-readable message from a wrapped domain error,
// stripping the sentinel kind prefix (e.g. "conflict: email_taken" -> "email_taken").
func Message(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	for _, kind := range []error{ErrNotFound, ErrConflict, ErrForbidden, ErrValidation, ErrUnauthorized} {
		prefix := kind.Error() + ": "
		if rest, ok := strings.CutPrefix(s, prefix); ok {
			return rest
		}
	}
	return s
}
