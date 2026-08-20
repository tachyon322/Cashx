// Package integration_test runs the CashX end-to-end integration suite
// against a real PostgreSQL (cashx_test) and in-process HTTP servers.
package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/thecodearcher/limen"

	"cashx/internal/api"
	"cashx/internal/auth"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	testPool *pgxpool.Pool
	adminDB  *sql.DB
	cfg      platform.Config
)

const allTables = `
users, sessions, verifications, staff_role_assignments,
partner_profiles, partner_referrals, partner_referral_clicks, media_assets,
projects, integration_keys, project_settings, offers, offer_terms_versions,
partner_offer_accesses, tracking_links, source_groups,
tracking_clicks, external_user_attributions, incoming_events, conversion_events,
daily_partner_offer_stats, daily_tracking_link_stats,
wallets, wallet_ledger_entries, commission_earnings, referral_rewards,
withdrawal_requests, payout_requisites, payout_transfers, payout_rules, platform_settings,
announcements, announcement_audiences, announcement_reads, user_notifications,
outbox_messages, audit_log`

// setup returns a ready pool with a clean database (migrations applied once,
// tables truncated per test).
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		adminURL := os.Getenv("CASHX_TEST_ADMIN_DATABASE_URL")
		appURL := os.Getenv("CASHX_TEST_DATABASE_URL")
		if adminURL == "" || appURL == "" {
			t.Fatal("CASHX_TEST_ADMIN_DATABASE_URL and CASHX_TEST_DATABASE_URL are required")
		}
		db, err := sql.Open("pgx", adminURL)
		if err != nil {
			t.Fatalf("open admin db: %v", err)
		}
		goose.SetBaseFS(os.DirFS("../../migrations"))
		if err := goose.SetDialect("postgres"); err != nil {
			t.Fatalf("dialect: %v", err)
		}
		if err := goose.Up(db, "."); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		adminDB = db
		pool, err := pgxpool.New(context.Background(), appURL)
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		testPool = pool
		cfg, err = platform.Load()
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		cfg.RateLimit = false
		cfg.Env = "development"
	}
	ctx := context.Background()
	if _, err := adminDB.ExecContext(ctx, "TRUNCATE "+allTables+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	// Restore the singleton seeds that migrations insert once (TRUNCATE
	// removed them); the app code treats their absence as a fatal error.
	if _, err := adminDB.ExecContext(ctx, `
INSERT INTO payout_rules (id) VALUES ('00000000-0000-0000-0000-000000000001') ON CONFLICT DO NOTHING;
INSERT INTO platform_settings (key, value) VALUES
    ('referral_rate_default_bps', '250'),
    ('referral_rate_max_bps', '500')
ON CONFLICT (key) DO NOTHING;`); err != nil {
		t.Fatalf("reseed singletons: %v", err)
	}
	seedTestAdmin(t)
	return testPool
}

// seedTestAdmin creates the superadmin used by the tests (mirrors the app seed).
func seedTestAdmin(t *testing.T) {
	t.Helper()
	q := repository.New(testPool)
	n, err := q.CountUsers(context.Background())
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n > 0 {
		return
	}
	limenAuth, err := auth.New(cfg, testPool)
	if err != nil {
		t.Fatalf("limen: %v", err)
	}
	pwd := cfg.AdminPassword
	res, err := limenAuth.Password.SignUpWithCredentialAndPassword(context.Background(),
		&limen.User{Email: cfg.AdminEmail, Password: &pwd},
		map[string]any{"name": "Администратор", "role": "staff", "is_active": true},
	)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := q.CreateStaffRoleAssignment(context.Background(), repository.CreateStaffRoleAssignmentParams{
		UserID: fmt.Sprint(res.User.ID), Role: "superadmin", ProjectID: repository.UUIDPtr(nil),
	}); err != nil {
		t.Fatalf("seed admin role: %v", err)
	}
	refCode, err := auth.GenerateReferralCode(context.Background(), q)
	if err != nil {
		t.Fatalf("seed admin referral code: %v", err)
	}
	profile, err := q.CreatePartnerProfile(context.Background(), repository.CreatePartnerProfileParams{
		UserID:       fmt.Sprint(res.User.ID),
		ReferralCode: refCode,
		ReferredBy:   repository.UUIDPtr(nil),
	})
	if err != nil {
		t.Fatalf("seed admin partner profile: %v", err)
	}
	approved := true
	if _, err := q.UpdatePartnerProfile(context.Background(), repository.UpdatePartnerProfileParams{
		ID: profile.ID, IsApproved: repository.BoolPtr(&approved),
	}); err != nil {
		t.Fatalf("approve admin partner profile: %v", err)
	}
	if _, err := q.CreateWallet(context.Background(), profile.ID); err != nil {
		t.Fatalf("seed admin wallet: %v", err)
	}
}

// newServers returns API and redirect httptest servers sharing the pool.
func newServers(t *testing.T, pool *pgxpool.Pool) (*httptest.Server, *httptest.Server) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	limenAuth, err := auth.New(cfg, pool)
	if err != nil {
		t.Fatalf("limen: %v", err)
	}
	srv := api.New(cfg, log, pool, limenAuth)
	apiServer := httptest.NewServer(srv.Router(nil))
	redirectServer := httptest.NewServer(tracking.NewRedirectHandler(tracking.RedirectDeps{
		Cfg: cfg, Log: log, Pool: pool, Redis: nil,
	}))
	t.Cleanup(apiServer.Close)
	t.Cleanup(redirectServer.Close)
	return apiServer, redirectServer
}

func newJar(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		if raw, ok := body.([]byte); ok {
			rd = bytes.NewReader(raw)
		} else {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			rd = bytes.NewReader(raw)
		}
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

// signEvent builds the X-CashX-* headers for a request body.
func signEvent(secret string, body []byte) map[string]string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	return map[string]string{
		"X-CashX-Key":       "",
		"X-CashX-Timestamp": ts,
		"X-CashX-Signature": hex.EncodeToString(mac.Sum(nil)),
	}
}

func adminLogin(t *testing.T, apiSrv *httptest.Server) *http.Client {
	t.Helper()
	client := newJar(t)
	resp, body := doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": cfg.AdminEmail, "password": cfg.AdminPassword,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login: %d %s", resp.StatusCode, body)
	}
	return client
}

type setupData struct {
	Admin       *http.Client
	Partner     *http.Client
	Project     string
	Offer       string
	KeyID       string
	Secret      string
	Code        string
	ClickToken  string
	PartnerID   string
	UserID      string
	RedirectURL string
}

// fullSetup wires an admin, a project/offer/key, an approved partner and the
// partner's tracking link code.
func fullSetup(t *testing.T, pool *pgxpool.Pool) (setupData, *httptest.Server, *httptest.Server) {
	t.Helper()
	apiSrv, redirSrv := newServers(t, pool)
	admin := adminLogin(t, apiSrv)

	// Project.
	resp, body := doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/projects", map[string]any{
		"slug": "kazik", "name": "Kazik", "destination_url": "http://localhost:3000",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.StatusCode, body)
	}
	var proj struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(body, &proj)

	// Offer at 40%.
	resp, body = doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/offers", map[string]any{
		"project_id": proj.Id, "name": "Kazik Default", "status": "active", "rate_bps": 4000,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create offer: %d %s", resp.StatusCode, body)
	}
	var offer struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(body, &offer)

	// Integration key.
	resp, body = doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/integration-keys", map[string]any{
		"project_id": proj.Id,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create key: %d %s", resp.StatusCode, body)
	}
	var key struct {
		KeyId  string `json:"key_id"`
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal(body, &key)

	// Partner registration + approval.
	partner := newJar(t)
	resp, body = doJSON(t, partner, "POST", apiSrv.URL+"/api/v1/auth/register", map[string]any{
		"name": "Partner One", "email": "p1@test.local", "password": "password123",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}
	var reg struct {
		User struct {
			Id string `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(body, &reg)
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/admin/partners?search=p1%40test.local", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list partners: %d %s", resp.StatusCode, body)
	}
	var plist struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &plist)
	partnerID := plist.Items[0].Id
	resp, body = doJSON(t, admin, "PATCH", apiSrv.URL+"/api/v1/admin/partners/"+partnerID, map[string]any{
		"is_approved": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: %d %s", resp.StatusCode, body)
	}

	// Partner login + join offer.
	resp, body = doJSON(t, partner, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "p1@test.local", "password": "password123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("partner login: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, partner, "POST", apiSrv.URL+"/api/v1/cabinet/offers/"+offer.Id+"/join", nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("join: %d %s", resp.StatusCode, body)
	}
	var joined struct {
		TrackingUrl string `json:"tracking_url"`
	}
	_ = json.Unmarshal(body, &joined)
	code := strings.TrimPrefix(joined.TrackingUrl, cfg.WebOrigin+"/c/")

	// Click through the redirect to get a signed click token. Do not follow
	// the redirect: the destination (WebOrigin) is not expected to be up in
	// tests, only the Location header matters.
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(redirSrv.URL + "/c/" + code)
	if err != nil {
		t.Fatal(err)
	}
	loc := resp.Header.Get("Location")
	resp.Body.Close()
	var clickToken string
	if i := strings.Index(loc, "click_token="); i >= 0 {
		clickToken = loc[i+len("click_token="):]
	}
	if clickToken == "" {
		t.Fatalf("no click token in redirect: %s", loc)
	}

	return setupData{
		Admin: admin, Partner: partner, Project: proj.Id, Offer: offer.Id,
		KeyID: key.KeyId, Secret: key.Secret, Code: code, ClickToken: clickToken,
		PartnerID: partnerID, UserID: reg.User.Id, RedirectURL: redirSrv.URL,
	}, apiSrv, redirSrv
}

// sendEvent posts a signed event and returns the HTTP status.
func sendEvent(t *testing.T, client *http.Client, url, keyID, secret string, body map[string]any) (*http.Response, []byte) {
	t.Helper()
	raw, _ := json.Marshal(body)
	headers := signEvent(secret, raw)
	headers["X-CashX-Key"] = keyID
	return doJSON(t, client, "POST", url, raw, headers)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if testPool != nil {
		testPool.Close()
	}
	if adminDB != nil {
		adminDB.Close()
	}
	os.Exit(code)
}
