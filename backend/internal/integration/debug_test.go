package integration_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/thecodearcher/limen"

	"cashx/internal/auth"
)

func TestSessionLookupDebug(t *testing.T) {
	pool := setup(t)
	limenAuth, err := auth.New(cfg, pool)
	if err != nil {
		t.Fatal(err)
	}
	apiSrv, _ := newServers(t, pool)

	// Create a staff user directly through Limen.
	pwd := "password123"
	_, err = limenAuth.Password.SignUpWithCredentialAndPassword(t.Context(),
		&limen.User{Email: "staff-debug@test.local", Password: &pwd},
		map[string]any{"name": "Debug", "role": "staff", "is_active": true},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Sign in over HTTP; the session cookie must resolve on /auth/me.
	client := newJar(t)
	resp, body := doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "staff-debug@test.local", "password": "password123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signin: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, client, "GET", apiSrv.URL+"/api/v1/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "staff-debug@test.local") {
		t.Fatalf("me: %d %s", resp.StatusCode, body)
	}
	t.Logf("session lookup ok: %s", body)
}
