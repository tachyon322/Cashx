package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// refCodeRe matches the short referral-code format shared by partners and
// staff: exactly 8 chars from the unambiguous alphabet.
var refCodeRe = regexp.MustCompile(`^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{8}$`)

func eventBase(extUser, eventID string) map[string]any {
	return map[string]any{
		"event_id":         eventID,
		"occurred_at":      time.Now().UTC().Format(time.RFC3339),
		"external_user_id": extUser,
	}
}

// wallet returns the partner's wallet from the DB.
func hmacSHA256(secret, data string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func wallet(t *testing.T, pool *pgxpool.Pool, partnerID string) (available, reserved int64) {
	t.Helper()
	err := pool.QueryRow(t.Context(), "SELECT available_kopecks, reserved_kopecks FROM wallets WHERE partner_id=$1", partnerID).Scan(&available, &reserved)
	if err != nil {
		t.Fatalf("wallet: %v", err)
	}
	return
}

func TestAuthFlow(t *testing.T) {
	pool := setup(t)
	apiSrv, _ := newServers(t, pool)
	client := newJar(t)

	// Register -> pending.
	resp, body := doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/register", map[string]any{
		"name": "New Partner", "email": "newp@test.local", "password": "password123",
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

	// Signin before approval succeeds (Limen creates the session), but the
	// cabinet is gated and me reports is_approved: false.
	resp, body = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "newp@test.local", "password": "password123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		// Limen rejects unknown/disabled accounts with 401; approval is not checked at signin.
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("signin before approval: want 200, got %d %s", resp.StatusCode, body)
		}
	}
	resp, body = doJSON(t, client, "GET", apiSrv.URL+"/api/v1/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"is_approved":false`) {
		t.Fatalf("me before approval: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, client, "GET", apiSrv.URL+"/api/v1/cabinet/summary", nil, nil)
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "pending_approval") {
		t.Fatalf("cabinet before approval: want 403 pending_approval, got %d %s", resp.StatusCode, body)
	}

	// Admin approves.
	admin := adminLogin(t, apiSrv)
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/cabinet/summary", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin cabinet summary: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/cabinet/offers", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin cabinet offers: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/admin/partners?search=newp%40test.local", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}
	var plist struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &plist)
	if len(plist.Items) == 0 {
		t.Fatal("partner not found")
	}
	resp, body = doJSON(t, admin, "PATCH", apiSrv.URL+"/api/v1/admin/partners/"+plist.Items[0].Id, map[string]any{"is_approved": true}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: %d %s", resp.StatusCode, body)
	}

	// Cabinet now works with the same session.
	resp, body = doJSON(t, client, "GET", apiSrv.URL+"/api/v1/cabinet/summary", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cabinet after approval: %d %s", resp.StatusCode, body)
	}

	// Signout -> session gone.
	resp, _ = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signout", nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("signout: %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, "GET", apiSrv.URL+"/api/v1/auth/me", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after signout: want 401, got %d", resp.StatusCode)
	}

	// Wrong password.
	resp, _ = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "newp@test.local", "password": "wrongpass",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: want 401, got %d", resp.StatusCode)
	}

	// Password reset round trip (dev returns the token).
	resp, body = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/password-reset/request", map[string]string{
		"email": "newp@test.local",
	}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("reset request: %d %s", resp.StatusCode, body)
	}
	var rr struct {
		ResetToken *string `json:"reset_token"`
	}
	_ = json.Unmarshal(body, &rr)
	if rr.ResetToken == nil {
		t.Fatal("dev reset token expected")
	}
	resp, body = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/password-reset/confirm", map[string]string{
		"token": *rr.ResetToken, "new_password": "newpassword123",
	}, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reset confirm: %d %s", resp.StatusCode, body)
	}
	resp, _ = doJSON(t, client, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "newp@test.local", "password": "newpassword123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signin after reset: %d", resp.StatusCode)
	}
}

func TestHMACSignature(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"
	body := eventBase("hmac-user", "evt-hmac-user-reg")
	body["type"] = "registration.created"
	body["click_token"] = data.ClickToken
	raw, _ := json.Marshal(body)

	// Correct signature -> 202.
	headers := signEvent(data.Secret, raw)
	headers["X-CashX-Key"] = data.KeyID
	resp, dataResp := doJSON(t, data.Partner, "POST", url, raw, headers)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("valid sig: want 202, got %d %s", resp.StatusCode, dataResp)
	}

	// Wrong secret -> 401.
	headers = signEvent("wrong-secret", raw)
	headers["X-CashX-Key"] = data.KeyID
	resp, _ = doJSON(t, data.Partner, "POST", url, raw, headers)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad sig: want 401, got %d", resp.StatusCode)
	}

	// Stale timestamp -> 401.
	ts := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	mac := hmacSHA256(data.Secret, ts+"."+string(raw))
	headers = map[string]string{
		"X-CashX-Key": data.KeyID, "X-CashX-Timestamp": ts,
		"X-CashX-Signature": fmt.Sprintf("%x", mac),
	}
	resp, _ = doJSON(t, data.Partner, "POST", url, raw, headers)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("stale ts: want 401, got %d", resp.StatusCode)
	}

	// Unknown key -> 401.
	headers = signEvent(data.Secret, raw)
	headers["X-CashX-Key"] = "unknown-key"
	resp, _ = doJSON(t, data.Partner, "POST", url, raw, headers)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown key: want 401, got %d", resp.StatusCode)
	}
}

func TestAttributionAndCommission(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// registration.created with the signed click token.
	ev := eventBase("user-1", "evt-user-1-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("registration: %d %s", resp.StatusCode, body)
	}

	// revenue.confirmed 1000.00 RUB = 100000 kopecks at 40% -> 40000.
	ev = eventBase("user-1", "evt-user-1-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-1"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("revenue: %d %s", resp.StatusCode, body)
	}
	avail, _ := wallet(t, pool, data.PartnerID)
	if avail != 40000 {
		t.Fatalf("wallet: want 40000, got %d", avail)
	}

	// Duplicate replay -> no double credit.
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "duplicate") {
		t.Fatalf("replay: want duplicate, got %d %s", resp.StatusCode, body)
	}
	avail, _ = wallet(t, pool, data.PartnerID)
	if avail != 40000 {
		t.Fatalf("wallet after replay: want 40000, got %d", avail)
	}

	// No attribution -> ignored.
	ev = eventBase("ghost", "evt-ghost-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-2"
	ev["amount_kopecks"] = 50000
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "no_attribution") {
		t.Fatalf("no attribution: %d %s", resp.StatusCode, body)
	}
}

func TestReferralReward(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)

	// Referral code of partner 1 (from setup).
	var ref struct {
		ReferralCode string `json:"referral_code"`
	}
	resp, body := doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/referrals", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("referrals: %d %s", resp.StatusCode, body)
	}
	_ = json.Unmarshal(body, &ref)

	// Partner 2 registers with the referral code.
	p2 := newJar(t)
	resp, body = doJSON(t, p2, "POST", apiSrv.URL+"/api/v1/auth/register", map[string]any{
		"name": "Partner Two", "email": "p2@test.local", "password": "password123",
		"referral_code": ref.ReferralCode,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register p2: %d %s", resp.StatusCode, body)
	}

	// Invalid referral code -> 400.
	resp, body = doJSON(t, data.Partner, "POST", apiSrv.URL+"/api/v1/auth/register", map[string]any{
		"name": "BadRef", "email": "badref@test.local", "password": "password123",
		"referral_code": "NOPE1234",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "invalid_referral_code") {
		t.Fatalf("invalid referral code: %d %s", resp.StatusCode, body)
	}
	// Each partner is invited at most once (UNIQUE invited_partner_id).
	// A second registration with the same code creates a separate invitee.

	// Approve p2, login, join, click, register, pay.
	admin := adminLogin(t, apiSrv)
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/admin/partners?search=p2%40test.local", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list p2: %d %s", resp.StatusCode, body)
	}
	var plist struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body, &plist)
	p2ID := plist.Items[0].Id
	resp, body = doJSON(t, admin, "PATCH", apiSrv.URL+"/api/v1/admin/partners/"+p2ID, map[string]any{"is_approved": true}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve p2: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, p2, "POST", apiSrv.URL+"/api/v1/auth/signin/credential", map[string]string{
		"credential": "p2@test.local", "password": "password123",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("p2 login: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, p2, "POST", apiSrv.URL+"/api/v1/cabinet/offers/"+data.Offer+"/join", nil, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("p2 join: %d %s", resp.StatusCode, body)
	}
	var joined struct {
		TrackingUrl string `json:"tracking_url"`
	}
	_ = json.Unmarshal(body, &joined)
	code := strings.TrimPrefix(joined.TrackingUrl, cfg.WebOrigin+"/c/")
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(data.RedirectURL + "/c/" + code)
	if err != nil {
		t.Fatal(err)
	}
	loc := resp.Header.Get("Location")
	resp.Body.Close()
	token := loc[strings.Index(loc, "click_token=")+len("click_token="):]

	ev := eventBase("user-2", "evt-user-2-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = token
	resp, body = sendEvent(t, p2, apiSrv.URL+"/api/v1/integrations/events", data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("p2 registration: %d %s", resp.StatusCode, body)
	}
	ev = eventBase("user-2", "evt-user-2-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-2"
	ev["amount_kopecks"] = 100000
	resp, body = sendEvent(t, p2, apiSrv.URL+"/api/v1/integrations/events", data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("p2 revenue: %d %s", resp.StatusCode, body)
	}

	// Referrer earns 2.5% of the 40000 commission = 1000 kopecks.
	avail, _ := wallet(t, pool, data.PartnerID)
	// Partner 1 already had 40000 from its own setup payment? No — fullSetup
	// does not send payments; only clicks. So the referrer balance is 1000.
	if avail != 1000 {
		t.Fatalf("referrer wallet: want 1000, got %d", avail)
	}

	// Reversal of p2's payment reverses the referrer reward too.
	ev = eventBase("user-2-rev", "evt-user-2-rev2")
	ev["type"] = "revenue.reversed"
	ev["external_payment_id"] = "pay-2"
	resp, body = sendEvent(t, p2, apiSrv.URL+"/api/v1/integrations/events", data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("p2 reversal: %d %s", resp.StatusCode, body)
	}
	avail, _ = wallet(t, pool, data.PartnerID)
	if avail != 0 {
		t.Fatalf("referrer wallet after reversal: want 0, got %d", avail)
	}
}

func TestStaffReferralCodeFormat(t *testing.T) {
	pool := setup(t)
	apiSrv, _ := newServers(t, pool)
	admin := adminLogin(t, apiSrv)

	resp, body := doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/cabinet/referrals", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin referrals: %d %s", resp.StatusCode, body)
	}
	var ref struct {
		ReferralCode string `json:"referral_code"`
		InviteURL    string `json:"invite_url"`
	}
	_ = json.Unmarshal(body, &ref)
	if !refCodeRe.MatchString(ref.ReferralCode) {
		t.Fatalf("staff referral code %q must match %s", ref.ReferralCode, refCodeRe.String())
	}
	if strings.Contains(ref.ReferralCode, "STAFF-") {
		t.Fatalf("staff referral code %q must not carry the STAFF- prefix", ref.ReferralCode)
	}
	if !strings.Contains(ref.InviteURL, "/invite/"+ref.ReferralCode) {
		t.Fatalf("invite url %q does not embed code %q", ref.InviteURL, ref.ReferralCode)
	}
}

func TestReversal(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	ev := eventBase("rev-user", "evt-rev-user-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("registration: %d %s", resp.StatusCode, body)
	}
	ev = eventBase("rev-user", "evt-rev-user-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-r1"
	ev["amount_kopecks"] = 100000
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("revenue: %d %s", resp.StatusCode, body)
	}
	if avail, _ := wallet(t, pool, data.PartnerID); avail != 40000 {
		t.Fatalf("wallet before reversal: want 40000, got %d", avail)
	}

	ev = eventBase("rev-user-rev", "evt-rev-user-rev2")
	ev["type"] = "revenue.reversed"
	ev["external_payment_id"] = "pay-r1"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("reversal: %d %s", resp.StatusCode, body)
	}
	if avail, _ := wallet(t, pool, data.PartnerID); avail != 0 {
		t.Fatalf("wallet after reversal: want 0, got %d", avail)
	}

	// Duplicate reversal -> no double debit.
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "duplicate") {
		t.Fatalf("replay reversal: want duplicate, got %d %s", resp.StatusCode, body)
	}
	if avail, _ := wallet(t, pool, data.PartnerID); avail != 0 {
		t.Fatalf("wallet after replay reversal: want 0, got %d", avail)
	}

	// Unknown payment -> ignored.
	ev = eventBase("rev-user-unknown", "evt-rev-user-unknown")
	ev["type"] = "revenue.reversed"
	ev["external_payment_id"] = "pay-nope"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "payment_not_found") {
		t.Fatalf("unknown payment: %d %s", resp.StatusCode, body)
	}
}

func TestConcurrentWithdrawals(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)

	// Lower the payout minimum so withdrawal requests succeed.
	rulesResp, rulesBody := doJSON(t, data.Admin, "PUT", apiSrv.URL+"/api/v1/admin/finance/rules", map[string]any{
		"min_withdraw_kopecks": 10000, "usdt_rate": 92, "sbp_fee_flat_kopecks": 0, "sbp_fee_percent_bps": 0,
	}, nil)
	if rulesResp.StatusCode != http.StatusOK {
		t.Fatalf("rules: %d %s", rulesResp.StatusCode, rulesBody)
	}

	// Fund the wallet with 1000.00 RUB via a confirmed payment.
	url := apiSrv.URL + "/api/v1/integrations/events"
	ev := eventBase("wd-user", "evt-wd-user-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("registration: %d %s", resp.StatusCode, body)
	}
	ev = eventBase("wd-user", "evt-wd-user-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-wd"
	ev["amount_kopecks"] = 250000 // 40% = 100000 kopecks
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("revenue: %d %s", resp.StatusCode, body)
	}

	// Two parallel requests of 70000 kopecks each: exactly one succeeds.
	type result struct {
		status int
		body   string
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			resp, body := doJSON(t, data.Partner, "POST", apiSrv.URL+"/api/v1/cabinet/payouts/requests", map[string]any{
				"method": "usdt", "amount_kopecks": 70000, "requisites": "T-testaddress",
			}, nil)
			results <- result{resp.StatusCode, string(body)}
		}()
	}
	okCount, failCount := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.status == http.StatusCreated {
			okCount++
		} else if r.status == http.StatusConflict {
			failCount++
		} else {
			t.Fatalf("unexpected status %d: %s", r.status, r.body)
		}
	}
	if okCount != 1 || failCount != 1 {
		t.Fatalf("want exactly one success and one conflict, got %d/%d", okCount, failCount)
	}
	avail, reserved := wallet(t, pool, data.PartnerID)
	if avail != 30000 || reserved != 70000 {
		t.Fatalf("wallet after requests: want avail 30000 reserved 70000, got %d/%d", avail, reserved)
	}

	// List requests to find the pending one.
	resp, body = doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/payouts", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("payouts list: %d %s", resp.StatusCode, body)
	}
	var pl struct {
		Requests []struct {
			Id     string `json:"id"`
			Status string `json:"status"`
		} `json:"requests"`
	}
	_ = json.Unmarshal(body, &pl)
	var pendingID string
	for _, r := range pl.Requests {
		if r.Status == "pending" {
			pendingID = r.Id
		}
	}
	if pendingID == "" {
		t.Fatal("no pending withdrawal found")
	}

	// Admin rejects -> reserve returned.
	admin := adminLogin(t, apiSrv)
	resp, body = doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/withdrawals/"+pendingID+"/decide", map[string]any{
		"decision": "rejected", "comment": "test",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reject: %d %s", resp.StatusCode, body)
	}
	avail, reserved = wallet(t, pool, data.PartnerID)
	if avail != 100000 || reserved != 0 {
		t.Fatalf("wallet after reject: want avail 100000 reserved 0, got %d/%d", avail, reserved)
	}

	// New request -> approve -> pay.
	resp, body = doJSON(t, data.Partner, "POST", apiSrv.URL+"/api/v1/cabinet/payouts/requests", map[string]any{
		"method": "usdt", "amount_kopecks": 100000, "requisites": "T-testaddress",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("request: %d %s", resp.StatusCode, body)
	}
	var created struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(body, &created)
	resp, body = doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/withdrawals/"+created.Id+"/decide", map[string]any{"decision": "approved"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/withdrawals/"+created.Id+"/pay", map[string]any{"external_tx_id": "tx-1"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pay: %d %s", resp.StatusCode, body)
	}
	avail, reserved = wallet(t, pool, data.PartnerID)
	if avail != 0 || reserved != 0 {
		t.Fatalf("wallet after pay: want 0/0, got %d/%d", avail, reserved)
	}

	// Below-minimum request.
	resp, body = doJSON(t, data.Partner, "POST", apiSrv.URL+"/api/v1/cabinet/payouts/requests", map[string]any{
		"method": "usdt", "amount_kopecks": 10, "requisites": "T-testaddress",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "below_min_withdraw") {
		t.Fatalf("below min: %d %s", resp.StatusCode, body)
	}
}

func TestAuditLog(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	admin := data.Admin

	// Rate change records an audit entry.
	resp, body := doJSON(t, admin, "POST", apiSrv.URL+"/api/v1/admin/partners/"+data.PartnerID+"/rate", map[string]any{
		"offer_id": data.Offer, "rate_bps": 3000,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rate: %d %s", resp.StatusCode, body)
	}
	// Rules update records an audit entry.
	resp, body = doJSON(t, admin, "PUT", apiSrv.URL+"/api/v1/admin/finance/rules", map[string]any{
		"min_withdraw_kopecks": 100000, "usdt_rate": 92, "sbp_fee_flat_kopecks": 0, "sbp_fee_percent_bps": 0,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rules: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/admin/audit?entity_type=partner", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "partner.rate_changed") {
		t.Fatalf("audit log missing rate change: %s", body)
	}
	resp, body = doJSON(t, admin, "GET", apiSrv.URL+"/api/v1/admin/audit?entity_type=payout_rules", nil, nil)
	if !strings.Contains(string(body), "payout_rules.updated") {
		t.Fatalf("audit log missing rules update: %s", body)
	}
}

func TestFinancialImmutability(t *testing.T) {
	pool := setup(t)
	_, apiSrv, _ := fullSetup(t, pool)
	_ = apiSrv

	// App role can INSERT into the ledger...
	partnerID := "00000000-0000-0000-0000-000000000001"
	// ...but UPDATE/DELETE must be rejected by grants.
	_, err := pool.Exec(t.Context(), "UPDATE wallet_ledger_entries SET amount_kopecks = 0 WHERE false")
	if err == nil {
		t.Fatal("UPDATE on wallet_ledger_entries must be denied for cashx_app")
	}
	_, err = pool.Exec(t.Context(), "DELETE FROM commission_earnings WHERE false")
	if err == nil {
		t.Fatal("DELETE on commission_earnings must be denied for cashx_app")
	}
	_ = partnerID
}

func TestE2E(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)

	// Lower the payout minimum so withdrawal requests succeed.
	admin := data.Admin
	rulesResp, rulesBody := doJSON(t, admin, "PUT", apiSrv.URL+"/api/v1/admin/finance/rules", map[string]any{
		"min_withdraw_kopecks": 10000, "usdt_rate": 92, "sbp_fee_flat_kopecks": 0, "sbp_fee_percent_bps": 0,
	}, nil)
	if rulesResp.StatusCode != http.StatusOK {
		t.Fatalf("rules: %d %s", rulesResp.StatusCode, rulesBody)
	}

	// Summary shows zero income before events.
	resp, body := doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/summary", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("summary: %d %s", resp.StatusCode, body)
	}
	var summary struct {
		Balance struct {
			AvailableKopecks int64 `json:"available_kopecks"`
		} `json:"balance"`
		ActiveOffers []any `json:"active_offers"`
	}
	_ = json.Unmarshal(body, &summary)
	if summary.Balance.AvailableKopecks != 0 {
		t.Fatalf("initial balance: want 0, got %d", summary.Balance.AvailableKopecks)
	}
	if len(summary.ActiveOffers) != 1 {
		t.Fatalf("active offers: want 1, got %d", len(summary.ActiveOffers))
	}

	// Full flow: click -> registration -> payment -> commission -> payout.
	url := apiSrv.URL + "/api/v1/integrations/events"
	ev := eventBase("e2e-user", "evt-e2e-user-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("registration: %d %s", resp.StatusCode, body)
	}
	ev = eventBase("e2e-user", "evt-e2e-user-rev")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "pay-e2e"
	ev["amount_kopecks"] = 100000
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if !strings.Contains(string(body), "accepted") {
		t.Fatalf("revenue: %d %s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/summary", nil, nil)
	_ = json.Unmarshal(body, &summary)
	if summary.Balance.AvailableKopecks != 40000 {
		t.Fatalf("balance after revenue: want 40000, got %d", summary.Balance.AvailableKopecks)
	}

	// Withdrawal request -> approve -> pay -> balance zero.
	resp, body = doJSON(t, data.Partner, "POST", apiSrv.URL+"/api/v1/cabinet/payouts/requests", map[string]any{
		"method": "usdt", "amount_kopecks": 40000, "requisites": "T-e2e",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("payout request: %d %s", resp.StatusCode, body)
	}
	var req struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(body, &req)
	resp, body = doJSON(t, data.Admin, "POST", apiSrv.URL+"/api/v1/admin/withdrawals/"+req.Id+"/decide", map[string]any{"decision": "approved"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("decide: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Admin, "POST", apiSrv.URL+"/api/v1/admin/withdrawals/"+req.Id+"/pay", map[string]any{"external_tx_id": "e2e-tx"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pay: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/payouts", nil, nil)
	if !strings.Contains(string(body), `"status":"paid"`) {
		t.Fatalf("payouts after pay: %s", body)
	}
	avail, _ := wallet(t, pool, data.PartnerID)
	if avail != 0 {
		t.Fatalf("final balance: want 0, got %d", avail)
	}

	// Notifications were created for the partner.
	resp, body = doJSON(t, data.Partner, "GET", apiSrv.URL+"/api/v1/cabinet/notifications", nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "unread_count") {
		t.Fatalf("notifications: %d %s", resp.StatusCode, body)
	}
}
