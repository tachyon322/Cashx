package tracking

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"cashx/internal/platform"
)

func TestNormalizeCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain code", "LITGWIN", "LITGWIN"},
		{"lowercase", "litgwin", "LITGWIN"},
		{"trailing space from a messenger link", "LITGWIN ", "LITGWIN"},
		{"leading space", " LITGWIN", "LITGWIN"},
		{"non-breaking space", "LITGWIN\u00a0", "LITGWIN"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCode(tc.in); got != tc.want {
				t.Fatalf("normalizeCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRedirectFallbackTarget pins the landing used when a click cannot be
// resolved: it follows RedirectFallback, and stays on FrontendOrigin only when
// that variable is unset (legacy behaviour). An oversize code returns before
// the database is touched, so the pool stays nil here.
func TestRedirectFallbackTarget(t *testing.T) {
	const oversize = "/c/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 33 chars > 32

	cases := []struct {
		name     string
		fallback string
		want     string
	}{
		{"configured casino landing", "https://litxgame.com", "https://litxgame.com"},
		{"unset keeps frontend origin", "", "https://cashxpay.cc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewRedirectHandler(RedirectDeps{
				Cfg: platform.Config{FrontendOrigin: "https://cashxpay.cc", RedirectFallback: tc.fallback},
				Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, oversize, nil))
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Fatalf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}
