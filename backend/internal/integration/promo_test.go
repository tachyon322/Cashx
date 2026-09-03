package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cashx/internal/tracking"
)

// TestPromoAttribution covers registration.created with a source/promo code
// instead of a click_token: the attribution must be tied to the tracking
// link itself, the response must describe the source (so the project can
// grant the registration bonus), and later revenue must credit the partner
// and land in per-source stats.
func TestPromoAttribution(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1"

	// Partner creates a promo source with a registration bonus.
	resp, body := doJSON(t, data.Partner, "POST", base+"/cabinet/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Promo code", "code": "PROMO777", "type": "promo", "registration_bonus": 7777,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create promo source: %d %s", resp.StatusCode, body)
	}
	var promo sourceItem
	_ = json.Unmarshal(body, &promo)
	if promo.Code != "PROMO777" {
		t.Fatalf("promo code: %+v", promo)
	}

	// Registration attributed by the promo code (no click_token).
	ev := eventBase("promo-user-1", "reg-promo-1")
	ev["type"] = "registration.created"
	ev["source_code"] = "PROMO777"
	resp, body = sendEvent(t, &http.Client{}, base+"/integrations/events", data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), `"status":"accepted"`) {
		t.Fatalf("promo registration: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"is_promo":true`) ||
		!strings.Contains(string(body), `"registration_bonus":7777`) ||
		!strings.Contains(string(body), `"code":"PROMO777"`) {
		t.Fatalf("promo registration response should describe the source: %s", body)
	}

	// Attribution row: no click, tracking_link_id set, partner/offer bound.
	var clickID *int64
	var linkID, partnerID *string
	err := pool.QueryRow(t.Context(),
		`SELECT tracking_click_id, tracking_link_id, partner_id FROM external_user_attributions WHERE external_user_id='promo-user-1'`,
	).Scan(&clickID, &linkID, &partnerID)
	if err != nil {
		t.Fatalf("attribution: %v", err)
	}
	if clickID != nil {
		t.Fatalf("promo attribution must not have a click, got %v", clickID)
	}
	if linkID == nil || *linkID != promo.ID {
		t.Fatalf("attribution tracking_link_id: want %s, got %v", promo.ID, linkID)
	}

	// Unknown code -> ignored.
	ev = eventBase("promo-user-2", "reg-promo-2")
	ev["type"] = "registration.created"
	ev["source_code"] = "NOSUCH"
	resp, body = sendEvent(t, &http.Client{}, base+"/integrations/events", data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "unknown_source_code") {
		t.Fatalf("unknown promo code: %d %s", resp.StatusCode, body)
	}

	// Neither click_token nor source_code -> ignored.
	ev = eventBase("promo-user-3", "reg-promo-3")
	ev["type"] = "registration.created"
	resp, body = sendEvent(t, &http.Client{}, base+"/integrations/events", data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "invalid_click_token") {
		t.Fatalf("no token no code: %d %s", resp.StatusCode, body)
	}

	// Deposit from the promo-referred user credits the partner and attributes
	// the earning to the promo link.
	_, before := wallet(t, pool, data.PartnerID)
	ev = eventBase("promo-user-1", "pay-promo-1")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-promo-1"
	ev["amount_kopecks"] = 100000
	resp, body = sendEvent(t, &http.Client{}, base+"/integrations/events", data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("revenue: %d %s", resp.StatusCode, body)
	}
	after, _ := wallet(t, pool, data.PartnerID)
	if after-before != 40000 { // 40% of 100000
		t.Fatalf("commission: want 40000, got %d", after-before)
	}
	var earnLink *string
	if err := pool.QueryRow(t.Context(),
		`SELECT tracking_link_id FROM commission_earnings WHERE external_user_id='promo-user-1'`,
	).Scan(&earnLink); err != nil {
		t.Fatalf("earning: %v", err)
	}
	if earnLink == nil || *earnLink != promo.ID {
		t.Fatalf("earning tracking_link_id: want %s, got %v", promo.ID, earnLink)
	}

	// Promo registration lands in per-source daily stats after recompute.
	from := time.Now().Add(-48 * time.Hour)
	to := time.Now().Add(24 * time.Hour)
	if err := tracking.RecomputeDailyStats(t.Context(), pool, from, to); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	var regs int
	if err := pool.QueryRow(t.Context(),
		`SELECT registrations FROM daily_tracking_link_stats WHERE tracking_link_id=$1`, promo.ID,
	).Scan(&regs); err != nil {
		t.Fatalf("daily link stats: %v", err)
	}
	if regs != 1 {
		t.Fatalf("daily link stats registrations: want 1, got %d", regs)
	}
}

// TestIntegrationsSourceLookup covers GET /integrations/source (HMAC): the
// signed project key resolves a source code inside its own project scope.
func TestIntegrationsSourceLookup(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1"

	// Unknown code -> 404.
	resp, body := doSignedGet(t, base+"/integrations/source?code="+data.Code, data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("source lookup: %d %s", resp.StatusCode, body)
	}
	var src struct {
		Code             string `json:"code"`
		Type             string `json:"type"`
		IsPromo          bool   `json:"is_promo"`
		IsActive         bool   `json:"is_active"`
		AccessActive     bool   `json:"access_active"`
		RegistrationBonus *int32 `json:"registration_bonus"`
	}
	if err := json.Unmarshal(body, &src); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if src.Code != data.Code || src.Type != "link" || src.IsPromo || !src.IsActive || !src.AccessActive {
		t.Fatalf("source lookup payload: %+v", src)
	}

	// Unknown code -> 404.
	resp, body = doSignedGet(t, base+"/integrations/source?code=NOPE", data.KeyID, data.Secret)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown code: want 404, got %d %s", resp.StatusCode, body)
	}

	// Unsigned -> 401.
	resp, _ = doJSON(t, &http.Client{}, "GET", base+"/integrations/source?code="+data.Code, nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsigned lookup: want 401, got %d", resp.StatusCode)
	}
}

// TestPartitionAutoCreate reproduces the 2026-09-01 incident shape and the
// self-healing fix: with the current-month partition missing, a raw insert
// fails with SQLSTATE 23514 (no trigger can create a partition for the
// table being inserted into), tracking.EnsurePartitionsFor restores it from
// the app role, and RecordClick retries automatically through the redirect.
func TestPartitionAutoCreate(t *testing.T) {
	pool := setup(t)
	data, _, redirSrv := fullSetup(t, pool)

	// Resolve the partner's link id (data.Code is the link code).
	var linkID string
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM tracking_links WHERE code=$1`, data.Code).Scan(&linkID); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Drop the current-month partition — the incident shape. DROP needs the
	// admin connection (partitions are owned by the migration role).
	curName := fmt.Sprintf("tracking_clicks_%s", time.Now().Format("2006_01"))
	if _, err := adminDB.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE IF EXISTS %s`, curName)); err != nil {
		t.Fatalf("drop partition: %v", err)
	}

	// Raw app-role insert fails: routing happens before any trigger could
	// act (this exact error killed live ingestion on 2026-09-01).
	_, err := pool.Exec(t.Context(),
		`INSERT INTO tracking_clicks (tracking_link_id, ip, created_at) VALUES ($1, '127.0.0.1'::inet, now())`, linkID)
	if err == nil || !strings.Contains(err.Error(), "23514") {
		t.Fatalf("raw insert into missing partition: want 23514, got %v", err)
	}

	// ensure_partitions_for (SECURITY DEFINER) works from the app role.
	if err := tracking.EnsurePartitionsFor(t.Context(), pool, time.Now()); err != nil {
		t.Fatalf("ensure_partitions_for from app role: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO tracking_clicks (tracking_link_id, ip, created_at) VALUES ($1, '127.0.0.1'::inet, now())`, linkID); err != nil {
		t.Fatalf("insert after ensure: %v", err)
	}

	// End-to-end self-heal: drop the partition again, then click through the
	// redirect — RecordClick must retry once and still return 302 with a
	// click token.
	if _, err := adminDB.ExecContext(t.Context(), fmt.Sprintf(`DROP TABLE IF EXISTS %s`, curName)); err != nil {
		t.Fatalf("drop partition again: %v", err)
	}
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(redirSrv.URL + "/c/" + data.Code)
	if err != nil {
		t.Fatal(err)
	}
	loc := resp.Header.Get("Location")
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || !strings.Contains(loc, "click_token=") {
		t.Fatalf("redirect after partition drop: %d %s", resp.StatusCode, loc)
	}
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM tracking_clicks WHERE tracking_link_id=$1`, linkID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n < 1 {
		t.Fatal("self-healed click was not recorded")
	}
}

// doSignedGet performs a GET with X-CashX-* HMAC headers.
func doSignedGet(t *testing.T, url, keyID, secret string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmacSHA256(secret, ts+".")
	req.Header.Set("X-CashX-Key", keyID)
	req.Header.Set("X-CashX-Timestamp", ts)
	req.Header.Set("X-CashX-Signature", fmt.Sprintf("%x", mac))
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}
