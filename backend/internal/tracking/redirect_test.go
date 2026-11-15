package tracking

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// TestRedirectFallbackByHost pins the per-project landing: a broken click that
// came in on an active offer domain returns to that casino, never to another
// project's casino; unknown hosts (incl. the CashX host for /c/ links) keep
// the configured fallback.
func TestRedirectFallbackByHost(t *testing.T) {
	const oversize = "/r/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 33 chars > 32
	domains := map[string]string{
		"starxplay.net": "https://starxplay.net",
		"litxgame.com":  "https://litxgame.com",
	}
	var seen []string
	resolve := func(_ context.Context, host string) (string, bool) {
		seen = append(seen, host)
		o, ok := domains[host]
		return o, ok
	}
	cases := []struct {
		name, host, want string
	}{
		{"starx domain stays on starx", "starxplay.net", "https://starxplay.net"},
		{"litgame domain stays on litgame", "litxgame.com", "https://litxgame.com"},
		{"cashx host keeps configured fallback", "cashxpay.cc", "https://litxgame.com"},
		{"unknown host keeps configured fallback", "evil.example", "https://litxgame.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen = nil
			h := NewRedirectHandler(RedirectDeps{
				Cfg:               platform.Config{FrontendOrigin: "https://cashxpay.cc", RedirectFallback: "https://litxgame.com"},
				Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
				ResolveHostOrigin: resolve,
			})
			req := httptest.NewRequest(http.MethodGet, oversize, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Fatalf("Location = %q, want %q", got, tc.want)
			}
			if len(seen) != 1 || seen[0] != tc.host {
				t.Fatalf("resolver saw %v, want [%s]", seen, tc.host)
			}
		})
	}
}

func TestHostOriginCandidates(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"starxplay.net", []string{"https://starxplay.net", "http://starxplay.net", "https://www.starxplay.net", "http://www.starxplay.net"}},
		{"WWW.StarXplay.net:443", []string{"https://www.starxplay.net", "http://www.starxplay.net", "https://starxplay.net", "http://starxplay.net"}},
		{"litxgame.com.", []string{"https://litxgame.com", "http://litxgame.com", "https://www.litxgame.com", "http://www.litxgame.com"}},
		{"", nil},
	}
	for _, tc := range cases {
		if got := hostOriginCandidates(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("hostOriginCandidates(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
