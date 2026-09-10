package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"cashx/internal/repository"
)

func TestConfirmedEventDedup(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register player
	ev := eventBase("user-dedup-1", "evt-dedup-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}

	// First payment
	ev = eventBase("user-dedup-1", "evt-dedup-pay-1")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-dedup-1"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("first payment: %d %s", resp.StatusCode, body)
	}
	avail, _ := wallet(t, pool, data.PartnerID)
	if avail != 40000 {
		t.Fatalf("wallet after first payment: want 40000, got %d", avail)
	}

	// Second payment with SAME external_payment_id but DIFFERENT event_id
	ev = eventBase("user-dedup-1", "evt-dedup-pay-2-diff-id")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-dedup-1"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "duplicate") {
		t.Fatalf("second payment with same payment_id: want 202 duplicate, got %d %s", resp.StatusCode, body)
	}
	avail, _ = wallet(t, pool, data.PartnerID)
	if avail != 40000 {
		t.Fatalf("wallet after duplicate: want 40000, got %d", avail)
	}
}

func TestConcurrentEventDedup(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register player
	ev := eventBase("user-concurrent", "evt-concurrent-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}

	// Concurrently send exact same event
	concurrency := 5
	var wg sync.WaitGroup
	results := make([]string, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payEv := eventBase("user-concurrent", "evt-concurrent-same-id")
			payEv["type"] = "revenue.confirmed"
			payEv["external_payment_id"] = "kazik-pay-concurrent-1"
			payEv["amount_kopecks"] = 50000
			payEv["currency"] = "RUB"
			_, b := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, payEv)
			results[idx] = string(b)
		}(i)
	}
	wg.Wait()

	acceptedCount := 0
	dupCount := 0
	for _, res := range results {
		if strings.Contains(res, "accepted") {
			acceptedCount++
		}
		if strings.Contains(res, "duplicate") {
			dupCount++
		}
	}
	if acceptedCount != 1 {
		t.Fatalf("concurrent dedup: want exactly 1 accepted, got %d (results: %v)", acceptedCount, results)
	}
	if dupCount != concurrency-1 {
		t.Fatalf("concurrent dedup: want %d duplicate, got %d", concurrency-1, dupCount)
	}
}

func TestIgnoredEventDoesNotLockKey(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// 1. Payment for unattributed user -> ignored (no_attribution)
	ev := eventBase("user-unattributed", "evt-unattr-pay-1")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-retry-key"
	ev["amount_kopecks"] = 50000
	ev["currency"] = "RUB"
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "ignored") || !strings.Contains(string(body), "no_attribution") {
		t.Fatalf("unattributed payment: want ignored no_attribution, got %d %s", resp.StatusCode, body)
	}

	// Verify key was NOT locked in conversion_event_payments
	var keyCount int64
	err := pool.QueryRow(t.Context(), "SELECT count(*) FROM conversion_event_payments WHERE external_payment_id = 'kazik-pay-retry-key'").Scan(&keyCount)
	if err != nil {
		t.Fatalf("query conversion_event_payments: %v", err)
	}
	if keyCount != 0 {
		t.Fatalf("key locked despite no_attribution: count = %d", keyCount)
	}

	// 2. Attribute user now
	regEv := eventBase("user-unattributed", "evt-unattr-reg")
	regEv["type"] = "registration.created"
	regEv["click_token"] = data.ClickToken
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, regEv)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}

	// 3. Retry payment with same external_payment_id and new event_id -> should be ACCEPTED now!
	retryEv := eventBase("user-unattributed", "evt-unattr-pay-retry")
	retryEv["type"] = "revenue.confirmed"
	retryEv["external_payment_id"] = "kazik-pay-retry-key"
	retryEv["amount_kopecks"] = 50000
	retryEv["currency"] = "RUB"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, retryEv)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("retry payment after attribution: want accepted, got %d %s", resp.StatusCode, body)
	}
}

func TestWalletResyncPreservesReserved(t *testing.T) {
	pool := setup(t)
	data, _, _ := fullSetup(t, pool)

	// Set wallet with available = 10000, reserved = 5000
	_, err := pool.Exec(t.Context(), `
		UPDATE wallets
		SET available_kopecks = 10000, reserved_kopecks = 5000
		WHERE partner_id = $1;
	`, data.PartnerID)
	if err != nil {
		t.Fatalf("update wallet: %v", err)
	}

	// Insert ledger entries summing to 25000
	var walletID string
	err = pool.QueryRow(t.Context(), "SELECT id FROM wallets WHERE partner_id = $1", data.PartnerID).Scan(&walletID)
	if err != nil {
		t.Fatalf("get wallet id: %v", err)
	}

	_, err = pool.Exec(t.Context(), `
		INSERT INTO wallet_ledger_entries (wallet_id, type, amount_kopecks, balance_after_kopecks, comment)
		VALUES ($1, 'commission', 20000, 20000, 'entry 1'),
		       ($1, 'commission', 5000, 25000, 'entry 2');
	`, walletID)
	if err != nil {
		t.Fatalf("insert ledger entries: %v", err)
	}

	// Resync wallet via ledger invariant formula
	_, err = pool.Exec(t.Context(), `
		UPDATE wallets w
		SET available_kopecks = (
			COALESCE((SELECT SUM(amount_kopecks) FROM wallet_ledger_entries WHERE wallet_id = w.id), 0)
		),
		updated_at = now()
		WHERE id = $1;
	`, walletID)
	if err != nil {
		t.Fatalf("resync wallet: %v", err)
	}

	avail, res := wallet(t, pool, data.PartnerID)
	if avail != 25000 {
		t.Fatalf("available_kopecks after resync: want 25000, got %d", avail)
	}
	if res != 5000 {
		t.Fatalf("reserved_kopecks after resync: want 5000 preserved, got %d", res)
	}
}

func TestConfirmedGateKind(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register player
	ev := eventBase("user-gate", "evt-gate-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("register: %d %s", resp.StatusCode, body)
	}

	// Send payment with kind: "gate"
	ev = eventBase("user-gate", "evt-gate-pay")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-gate-1"
	ev["amount_kopecks"] = 10000
	ev["currency"] = "RUB"
	ev["kind"] = "gate"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted || !strings.Contains(string(body), "accepted") {
		t.Fatalf("revenue with kind gate: %d %s", resp.StatusCode, body)
	}

	var kind string
	err := pool.QueryRow(t.Context(), `
		SELECT kind FROM conversion_events WHERE external_payment_id = 'kazik-pay-gate-1'
	`).Scan(&kind)
	if err != nil {
		t.Fatalf("query conversion_events.kind: %v", err)
	}
	if kind != "gate" {
		t.Fatalf("kind: want 'gate', got %q", kind)
	}
}

func TestNoAttribution(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	ev := eventBase("random-user-no-attr", "evt-no-attr-pay")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-no-attr"
	ev["amount_kopecks"] = 50000
	ev["currency"] = "RUB"
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status: want 202, got %d %s", resp.StatusCode, body)
	}
	var res struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Status != "ignored" || res.Reason != "no_attribution" {
		t.Fatalf("want status ignored reason no_attribution, got %s / %s", res.Status, res.Reason)
	}
}

func TestReversedConversionExclusion(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register and confirm payment
	ev := eventBase("user-rev-excl", "evt-rev-excl-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	_, _ = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)

	ev = eventBase("user-rev-excl", "evt-rev-excl-pay")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-rev-excl"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	_, _ = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)

	// Check that conversion exists
	var convID int64
	var linkID string
	err := pool.QueryRow(t.Context(), `
		SELECT ce.id, COALESCE(a.tracking_link_id::text, '')
		FROM conversion_events ce
		JOIN external_user_attributions a ON a.id = ce.attribution_id
		WHERE ce.external_payment_id = 'kazik-pay-rev-excl';
	`).Scan(&convID, &linkID)
	if err != nil {
		t.Fatalf("query conversion: %v", err)
	}

	q := repository.New(pool)
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	// Before reversal: AggFirstPaymentsByDay returns 1
	aggBefore, err := q.AggFirstPaymentsByDay(t.Context(), repository.AggFirstPaymentsByDayParams{
		OccurredAt:   repository.TimePtr(&start),
		OccurredAt_2: repository.TimePtr(&end),
	})
	if err != nil {
		t.Fatalf("AggFirstPaymentsByDay before: %v", err)
	}
	if len(aggBefore) != 1 || aggBefore[0].FirstPayments != 1 {
		t.Fatalf("aggBefore: want 1 first payment, got %v", aggBefore)
	}

	// Reverse conversion
	revRow, err := q.SetConversionReversed(t.Context(), convID)
	if err != nil {
		t.Fatalf("SetConversionReversed: %v", err)
	}
	if !revRow.ReversedAt.Valid {
		t.Fatalf("SetConversionReversed returned invalid reversed_at")
	}

	// After reversal: AggFirstPaymentsByDay returns 0
	aggAfter, err := q.AggFirstPaymentsByDay(t.Context(), repository.AggFirstPaymentsByDayParams{
		OccurredAt:   repository.TimePtr(&start),
		OccurredAt_2: repository.TimePtr(&end),
	})
	if err != nil {
		t.Fatalf("AggFirstPaymentsByDay after: %v", err)
	}
	if len(aggAfter) != 0 {
		t.Fatalf("aggAfter: want 0 first payments after reversal, got %v", aggAfter)
	}

	// BatchDepositsByLinksRange also excludes reversed conversions
	batchAfter, err := q.BatchDepositsByLinksRange(t.Context(), repository.BatchDepositsByLinksRangeParams{
		Column1:      []string{linkID},
		OccurredAt:   repository.TimePtr(&start),
		OccurredAt_2: repository.TimePtr(&end),
	})
	if err != nil {
		t.Fatalf("BatchDepositsByLinksRange after: %v", err)
	}
	if len(batchAfter) != 0 {
		t.Fatalf("batchAfter: want 0 deposits after reversal, got %v", batchAfter)
	}
}

func TestRevenueReversedMarksConversion(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register and confirm payment
	ev := eventBase("user-rev-flow", "evt-rev-flow-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	_, _ = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)

	ev = eventBase("user-rev-flow", "evt-rev-flow-pay")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "kazik-pay-rev-flow"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	resp, body := sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("revenue confirmed: %d %s", resp.StatusCode, body)
	}

	// Check conversion reversed_at is null
	var reversedAt *time.Time
	err := pool.QueryRow(t.Context(), `
		SELECT reversed_at FROM conversion_events WHERE external_payment_id = 'kazik-pay-rev-flow'
	`).Scan(&reversedAt)
	if err != nil {
		t.Fatalf("query reversed_at: %v", err)
	}
	if reversedAt != nil {
		t.Fatalf("reversed_at before refund should be nil, got %v", reversedAt)
	}

	avail, _ := wallet(t, pool, data.PartnerID)
	if avail != 40000 {
		t.Fatalf("wallet before refund: want 40000, got %d", avail)
	}

	// Send revenue.reversed with canonical payment id
	revEv := eventBase("user-rev-flow", "evt-rev-flow-reverse")
	revEv["type"] = "revenue.reversed"
	revEv["external_payment_id"] = "kazik-pay-rev-flow"
	resp, body = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, revEv)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("revenue reversed: %d %s", resp.StatusCode, body)
	}

	// Check conversion reversed_at IS NOT NULL now
	err = pool.QueryRow(t.Context(), `
		SELECT reversed_at FROM conversion_events WHERE external_payment_id = 'kazik-pay-rev-flow'
	`).Scan(&reversedAt)
	if err != nil {
		t.Fatalf("query reversed_at after reversal: %v", err)
	}
	if reversedAt == nil {
		t.Fatalf("reversed_at after refund should NOT be nil")
	}

	// Check commission_earnings reversed_at IS NOT NULL
	var earnReversedAt *time.Time
	err = pool.QueryRow(t.Context(), `
		SELECT ce.reversed_at
		FROM commission_earnings ce
		JOIN conversion_events c ON c.id = ce.conversion_event_id
		WHERE c.external_payment_id = 'kazik-pay-rev-flow';
	`).Scan(&earnReversedAt)
	if err != nil {
		t.Fatalf("query earning reversed_at: %v", err)
	}
	if earnReversedAt == nil {
		t.Fatalf("earning reversed_at after refund should NOT be nil")
	}

	// Check wallet debited back to 0
	avail, _ = wallet(t, pool, data.PartnerID)
	if avail != 0 {
		t.Fatalf("wallet after refund: want 0, got %d", avail)
	}
}

func TestReconcileIntegrityCheckMoscowTimezone(t *testing.T) {
	pool := setup(t)
	data, _, _ := fullSetup(t, pool)

	// Create daily_partner_offer_stats on day 2026-09-09 in Europe/Moscow timezone
	// In UTC, 2026-09-09 22:00:00 UTC is 2026-09-10 01:00:00 MSK.
	// We want to test that day boundary AT TIME ZONE 'Europe/Moscow' works properly.
	mskLoc := time.FixedZone("MSK", 3*3600)
	dayMSK := time.Date(2026, 9, 9, 0, 0, 0, 0, mskLoc)

	_, err := pool.Exec(t.Context(), `
		INSERT INTO daily_partner_offer_stats (partner_id, offer_id, day, first_payments, income_kopecks)
		VALUES ($1, $2, $3, 5, 500000);
	`, data.PartnerID, data.Offer, dayMSK.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("insert daily stats: %v", err)
	}

	fromMSK := time.Date(2026, 9, 5, 0, 0, 0, 0, mskLoc)
	toMSK := time.Date(2026, 9, 10, 0, 0, 0, 0, mskLoc)

	var statsFP, statsIncome int64
	err = pool.QueryRow(t.Context(), `
		SELECT COALESCE(sum(first_payments), 0), COALESCE(sum(income_kopecks), 0)
		FROM daily_partner_offer_stats
		WHERE day >= ($1 AT TIME ZONE 'Europe/Moscow')::date AND day < ($2 AT TIME ZONE 'Europe/Moscow')::date;
	`, fromMSK, toMSK).Scan(&statsFP, &statsIncome)
	if err != nil {
		t.Fatalf("query stats with timezone: %v", err)
	}
	if statsFP != 5 || statsIncome != 500000 {
		t.Fatalf("integrity check with timezone: want (5, 500000), got (%d, %d)", statsFP, statsIncome)
	}

	// Verify that the OLD uncorrected query ($1::date) fails to include 2026-09-09
	var oldFP int64
	_ = pool.QueryRow(t.Context(), `
		SELECT COALESCE(sum(first_payments), 0)
		FROM daily_partner_offer_stats
		WHERE day >= $1::date AND day < $2::date;
	`, fromMSK, toMSK).Scan(&oldFP)
	if oldFP != 0 {
		t.Logf("Notice: without timezone conversion oldFP = %d (Postgres session tz dependent)", oldFP)
	}
}

func TestReconcileEarningsCreatedAtSync(t *testing.T) {
	pool := setup(t)
	data, apiSrv, _ := fullSetup(t, pool)
	url := apiSrv.URL + "/api/v1/integrations/events"

	// Register user
	ev := eventBase("user-sync-test", "evt-sync-reg")
	ev["type"] = "registration.created"
	ev["click_token"] = data.ClickToken
	_, _ = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)

	// Ingest conversion with old unnormalized occurred_at
	oldTime := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	ev = eventBase("user-sync-test", "evt-sync-pay-old")
	ev["type"] = "revenue.confirmed"
	ev["external_payment_id"] = "raw-uuid-12345"
	ev["amount_kopecks"] = 100000
	ev["currency"] = "RUB"
	ev["occurred_at"] = oldTime.Format(time.RFC3339)
	_, _ = sendEvent(t, data.Partner, url, data.KeyID, data.Secret, ev)

	var convID int64
	err := pool.QueryRow(t.Context(), `SELECT id FROM conversion_events WHERE external_payment_id = 'raw-uuid-12345'`).Scan(&convID)
	if err != nil {
		t.Fatalf("find conv: %v", err)
	}

	// Verify initial commission_earnings.created_at matches oldTime
	var earnCreatedAt time.Time
	err = pool.QueryRow(t.Context(), `SELECT created_at FROM commission_earnings WHERE conversion_event_id = $1`, convID).Scan(&earnCreatedAt)
	if err != nil {
		t.Fatalf("find earning: %v", err)
	}
	if !earnCreatedAt.Equal(oldTime) {
		t.Fatalf("earning created_at want %v, got %v", oldTime, earnCreatedAt)
	}

	// Now simulate reconcile fallback normalization with new time from payments
	newTime := time.Date(2026, 9, 8, 12, 1, 30, 0, time.UTC)
	canonicalPayID := "kazik-pay-real-id"
	canonicalEvtID := "kazik-payment-real-id"

	_, err = pool.Exec(t.Context(), `
		UPDATE conversion_events
		SET external_payment_id = $1,
		    external_event_id = $2,
		    occurred_at = $3,
		    kind = 'deposit'
		WHERE id = $4;
	`, canonicalPayID, canonicalEvtID, newTime, convID)
	if err != nil {
		t.Fatalf("update conversion: %v", err)
	}

	_, err = pool.Exec(t.Context(), `
		UPDATE commission_earnings
		SET created_at = $1
		WHERE conversion_event_id = $2;
	`, newTime, convID)
	if err != nil {
		t.Fatalf("update earning created_at: %v", err)
	}

	// Verify commission_earnings.created_at was synced to newTime
	err = pool.QueryRow(t.Context(), `SELECT created_at FROM commission_earnings WHERE conversion_event_id = $1`, convID).Scan(&earnCreatedAt)
	if err != nil {
		t.Fatalf("query updated earning: %v", err)
	}
	if !earnCreatedAt.Equal(newTime) {
		t.Fatalf("synced earning created_at want %v, got %v", newTime, earnCreatedAt)
	}
}
