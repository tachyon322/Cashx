package httpjson

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the canonical error shape returned by the API.
type ErrorBody struct {
	Message string `json:"message"`
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// Error writes the canonical error response.
func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorBody{Message: message})
}

// Decode reads and validates a JSON request body into v.
func Decode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
