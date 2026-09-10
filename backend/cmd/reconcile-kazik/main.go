package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/tracking"
)

type ReconcileReport struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Project     string         `json:"project"`
	ProjectID   string         `json:"project_id"`
	Window      WindowReport   `json:"window"`
	Mode        string         `json:"mode"`
	Casino      CasinoReport   `json:"casino"`
	Cashx       CashxReport    `json:"cashx"`
	Matching    MatchingReport `json:"matching"`
	FraudAudit  []FraudEntry   `json:"fraud_audit"`
	Wallets     []WalletChange `json:"wallets"`
	Integrity   IntegrityCheck `json:"integrity_check"`
	Errors      []string       `json:"errors"`
	Warnings    []string       `json:"warnings"`
}

type WindowReport struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type CasinoReport struct {
	TotalPayments        int                 `json:"total_payments"`
	TotalAmountKopecks   int64               `json:"total_amount_kopecks"`
	TotalAmountRubles    float64             `json:"total_amount_rubles"`
	AttributedCount      int                 `json:"attributed_count"`
	AttributedSumKopecks int64               `json:"attributed_sum_kopecks"`
	UnattributedCount    int                 `json:"unattributed_count"`
	UnattributedSum      int64               `json:"unattributed_sum_kopecks"`
	Unattributed         []PaymentMovement   `json:"unattributed_movements"`
}

type CashxReport struct {
	InitialConversionsCount int64   `json:"initial_conversions_count"`
	InitialSumKopecks       int64   `json:"initial_sum_kopecks"`
	ActiveConversionsCount  int64   `json:"active_conversions_count"`
	ActiveSumKopecks        int64   `json:"active_sum_kopecks"`
	ActiveSumRubles         float64 `json:"active_sum_rubles"`
}

type MatchingReport struct {
	MatchedExactCount    int               `json:"matched_exact_count"`
	MatchedExactKopecks  int64             `json:"matched_exact_kopecks"`
	MatchedFallbackCount int               `json:"matched_fallback_count"`
	MatchedFallbackKop   int64             `json:"matched_fallback_kopecks"`
	MissingCount         int               `json:"missing_count"`
	MissingKopecks       int64             `json:"missing_kopecks"`
	OrphanCount          int               `json:"orphan_count"`
	OrphanKopecks        int64             `json:"orphan_kopecks"`
	MissingDetails       []PaymentMovement `json:"missing_details,omitempty"`
	OrphanDetails        []ConversionEntry `json:"orphan_details,omitempty"`
}

type PaymentMovement struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	AmountKopecks int64     `json:"amount_kopecks"`
	Purpose       string    `json:"purpose"`
	Kind          string    `json:"kind"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ConversionEntry struct {
	ID                int64      `json:"id"`
	ExternalPaymentID string     `json:"external_payment_id"`
	ExternalEventID   string     `json:"external_event_id"`
	ExternalUserID    string     `json:"external_user_id"`
	AmountKopecks     int64      `json:"amount_kopecks"`
	OccurredAt        time.Time  `json:"occurred_at"`
	Kind              string     `json:"kind"`
	ReversedAt        *time.Time `json:"reversed_at,omitempty"`
	ProcessingNote    string     `json:"processing_note"`
}

type FraudEntry struct {
	ExternalUserID string `json:"external_user_id"`
	EventsCount    int64  `json:"events_count"`
	TotalKopecks   int64  `json:"total_kopecks"`
}

type WalletChange struct {
	WalletID         string `json:"wallet_id"`
	PartnerID        string `json:"partner_id"`
	BeforeAvailable  int64  `json:"before_available_kopecks"`
	AfterAvailable   int64  `json:"after_available_kopecks"`
	ReservedKopecks  int64  `json:"reserved_kopecks"`
}

type IntegrityCheck struct {
	ActiveConversionsCount int64 `json:"active_conversions_count"`
	ActiveConversionsSum   int64 `json:"active_conversions_sum_kopecks"`
	ActiveEarningsSum      int64 `json:"active_earnings_sum_kopecks"`
	StatsFirstPayments     int64 `json:"stats_first_payments"`
	StatsIncomeKopecks     int64 `json:"stats_income_kopecks"`
	Passed                 bool  `json:"passed"`
}

type CasinoPayment struct {
	ID        string
	UserID    string
	AmountRub int
	Purpose   string
	UpdatedAt time.Time
}

type Attribution struct {
	ID             int64
	PartnerID      string
	OfferID        string
	TrackingLinkID *string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseTimeWindow(fromStr, toStr string) (time.Time, time.Time, time.Time, error) {
	mskLoc := time.FixedZone("MSK", 3*3600)
	var from, to time.Time
	var err error

	if fromStr == "" {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("-from is required")
	}
	if toStr == "" {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("-to is required")
	}

	if len(fromStr) == 10 {
		from, err = time.ParseInLocation("2006-01-02", fromStr, mskLoc)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("parse -from: %w", err)
		}
	} else {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("parse -from (RFC3339): %w", err)
		}
	}

	var tomorrowMSK time.Time
	if len(toStr) == 10 {
		t, err := time.ParseInLocation("2006-01-02", toStr, mskLoc)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("parse -to: %w", err)
		}
		to = t.AddDate(0, 0, 1) // strictly [from, to + 1 day 00:00 MSK)
		tomorrowMSK = to
	} else {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("parse -to (RFC3339): %w", err)
		}
		// Expand to next day 00:00 MSK for RecomputeDailyStats protection
		toInMSK := to.In(mskLoc)
		tomorrowMSK = time.Date(toInMSK.Year(), toInMSK.Month(), toInMSK.Day(), 0, 0, 0, 0, mskLoc).AddDate(0, 0, 1)
	}

	realTomorrowMSK := tracking.StartOfMSKDay(time.Now()).AddDate(0, 0, 1)
	if tomorrowMSK.Before(realTomorrowMSK) {
		tomorrowMSK = realTomorrowMSK
	}

	return from, to, tomorrowMSK, nil
}

func main() {
	defaultKazikDSN := envOr("KAZIK_DATABASE_URL", "postgres://kazik:kazik@localhost:5435/kazik")
	defaultCashxDSN := envOr("CASHX_ADMIN_DATABASE_URL", envOr("CASHX_DATABASE_URL", "postgres://cashx_app:cashx_app_dev_password@localhost:5436/cashx"))

	kazikDSN := flag.String("kazik-dsn", defaultKazikDSN, "Kazik PostgreSQL connection DSN")
	cashxDSN := flag.String("cashx-dsn", defaultCashxDSN, "Cashx PostgreSQL connection DSN")
	projectSlug := flag.String("project", "kazik", "Cashx project slug")
	fromStr := flag.String("from", "", "Start date (YYYY-MM-DD or RFC3339)")
	toStr := flag.String("to", "", "End date (YYYY-MM-DD or RFC3339)")
	apply := flag.Bool("apply", false, "Apply changes (default false = dry run)")
	reportPath := flag.String("report", "reconcile_report.json", "Report output path")
	attrTo := flag.String("attribute-unattributed-to", "", "Partner UUID to attribute unattributed payments to")
	attrUsersCSV := flag.String("attribute-unattributed-users", "", "Strict comma-separated allowlist of user IDs for attribution")
	flag.Parse()

	if *apply && (*fromStr == "" || *toStr == "") {
		log.Fatal("-from and -to are required when -apply is true")
	}

	from, to, tomorrowMSK, err := parseTimeWindow(*fromStr, *toStr)
	if err != nil {
		log.Fatalf("time window: %v", err)
	}

	ctx := context.Background()

	// Parse allowlist for unattributed users
	allowlistUsers := make(map[string]bool)
	if *attrUsersCSV != "" {
		for _, u := range strings.Split(*attrUsersCSV, ",") {
			trimmed := strings.TrimSpace(u)
			if trimmed != "" {
				allowlistUsers[trimmed] = true
			}
		}
	}
	if *attrTo != "" && len(allowlistUsers) == 0 {
		log.Fatal("flag --attribute-unattributed-to requires explicit non-empty --attribute-unattributed-users allowlist")
	}

	// Connect to Kazik DB
	kazikPool, err := pgxpool.New(ctx, *kazikDSN)
	if err != nil {
		log.Fatalf("connect kazik: %v", err)
	}
	defer kazikPool.Close()

	if err := kazikPool.Ping(ctx); err != nil {
		log.Fatalf("ping kazik db: %v", err)
	}

	// Connect to Cashx DB
	cashxPool, err := pgxpool.New(ctx, *cashxDSN)
	if err != nil {
		log.Fatalf("connect cashx: %v", err)
	}
	defer cashxPool.Close()

	if err := cashxPool.Ping(ctx); err != nil {
		log.Fatalf("ping cashx db: %v", err)
	}

	// Resolve project in Cashx
	var projectID, projectName string
	err = cashxPool.QueryRow(ctx, "SELECT id, name FROM projects WHERE slug = $1", *projectSlug).Scan(&projectID, &projectName)
	if err != nil {
		log.Fatalf("find project %q: %v", *projectSlug, err)
	}

	mode := "dry-run"
	if *apply {
		mode = "applied"
	}

	report := ReconcileReport{
		GeneratedAt: time.Now().UTC(),
		Project:     *projectSlug,
		ProjectID:   projectID,
		Window: WindowReport{
			From: from.Format(time.RFC3339),
			To:   to.Format(time.RFC3339),
		},
		Mode: mode,
	}

	fmt.Printf("=== Kazik Reconcile (%s) ===\n", mode)
	fmt.Printf("Project: %s (%s)\n", projectName, projectID)
	fmt.Printf("Window:  [%s, %s)\n", from.Format(time.RFC3339), to.Format(time.RFC3339))

	// 1. Fetch movements from casino
	casinoRows, err := kazikPool.Query(ctx, `
		SELECT id, user_id, amount, purpose, updated_at
		FROM payments
		WHERE status = 'PAID' AND credited = true AND amount > 0
		  AND updated_at >= $1 AND updated_at < $2
		ORDER BY updated_at;
	`, from, to)
	if err != nil {
		log.Fatalf("query kazik payments: %v", err)
	}
	defer casinoRows.Close()

	var payments []CasinoPayment
	var totalCasinoKopecks int64
	for casinoRows.Next() {
		var p CasinoPayment
		if err := casinoRows.Scan(&p.ID, &p.UserID, &p.AmountRub, &p.Purpose, &p.UpdatedAt); err != nil {
			log.Fatalf("scan kazik payment: %v", err)
		}
		payments = append(payments, p)
		totalCasinoKopecks += int64(p.AmountRub) * 100
	}
	report.Casino.TotalPayments = len(payments)
	report.Casino.TotalAmountKopecks = totalCasinoKopecks
	report.Casino.TotalAmountRubles = float64(totalCasinoKopecks) / 100.0

	// 2. Fetch existing conversions in Cashx window
	convRows, err := cashxPool.Query(ctx, `
		SELECT id, attribution_id, external_user_id, external_payment_id, external_event_id,
		       amount_kopecks, occurred_at, kind, reversed_at, COALESCE(processing_note, '')
		FROM conversion_events
		WHERE project_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		ORDER BY occurred_at;
	`, projectID, from, to)
	if err != nil {
		log.Fatalf("query cashx conversions: %v", err)
	}
	defer convRows.Close()

	var conversions []ConversionEntry
	var initialSumKopecks int64
	for convRows.Next() {
		var c ConversionEntry
		if err := convRows.Scan(&c.ID, new(int64), &c.ExternalUserID, &c.ExternalPaymentID, &c.ExternalEventID,
			&c.AmountKopecks, &c.OccurredAt, &c.Kind, &c.ReversedAt, &c.ProcessingNote); err != nil {
			log.Fatalf("scan cashx conversion: %v", err)
		}
		conversions = append(conversions, c)
		initialSumKopecks += c.AmountKopecks
	}
	report.Cashx.InitialConversionsCount = int64(len(conversions))
	report.Cashx.InitialSumKopecks = initialSumKopecks

	// 3. Fraud audit
	fraudRows, err := cashxPool.Query(ctx, `
		SELECT payload->>'external_user_id', count(*), COALESCE(sum((payload->>'amount_kopecks')::bigint), 0)
		FROM incoming_events
		WHERE project_id = $1 AND status = 'ignored' AND reason = 'no_attribution'
		  AND received_at >= $2 AND received_at < $3
		GROUP BY payload->>'external_user_id'
		ORDER BY count(*) DESC;
	`, projectID, from, to)
	if err == nil {
		defer fraudRows.Close()
		for fraudRows.Next() {
			var fe FraudEntry
			var uid *string
			if err := fraudRows.Scan(&uid, &fe.EventsCount, &fe.TotalKopecks); err == nil {
				if uid != nil {
					fe.ExternalUserID = *uid
				}
				report.FraudAudit = append(report.FraudAudit, fe)
			}
		}
	}

	// 4. Attribution check and safe attribution block
	type attributedPayment struct {
		payment CasinoPayment
		attr    Attribution
		kind    string
	}
	var attributedPayments []attributedPayment
	var pendingAttributionPayments []CasinoPayment

	// Find active offer for project if needed for attribution
	var defaultOfferID string
	_ = cashxPool.QueryRow(ctx, "SELECT id FROM offers WHERE project_id = $1 AND status = 'active' ORDER BY created_at ASC LIMIT 1", projectID).Scan(&defaultOfferID)

	for _, p := range payments {
		kind := "deposit"
		if p.Purpose != "deposit" {
			kind = "gate"
		}
		var attr Attribution
		var linkID *string
		err := cashxPool.QueryRow(ctx, `
			SELECT id, partner_id, offer_id, tracking_link_id
			FROM external_user_attributions
			WHERE project_id = $1 AND external_user_id = $2
			ORDER BY first_seen_at ASC LIMIT 1;
		`, projectID, p.UserID).Scan(&attr.ID, &attr.PartnerID, &attr.OfferID, &linkID)
		if err == nil {
			attr.TrackingLinkID = linkID
			attributedPayments = append(attributedPayments, attributedPayment{payment: p, attr: attr, kind: kind})
		} else if *attrTo != "" && allowlistUsers[p.UserID] {
			pendingAttributionPayments = append(pendingAttributionPayments, p)
		} else {
			report.Casino.Unattributed = append(report.Casino.Unattributed, PaymentMovement{
				ID:            p.ID,
				UserID:        p.UserID,
				AmountKopecks: int64(p.AmountRub) * 100,
				Purpose:       p.Purpose,
				Kind:          kind,
				UpdatedAt:     p.UpdatedAt,
			})
		}
	}

	// Check safe threshold for unattributed payments to attribute
	if len(pendingAttributionPayments) > 0 {
		var pendingSumKopecks int64
		for _, p := range pendingAttributionPayments {
			pendingSumKopecks += int64(p.AmountRub) * 100
		}
		if len(pendingAttributionPayments) > 3 || pendingSumKopecks > 1000000 { // 10 000 ₽ = 1 000 000 kopecks
			log.Fatalf("safety threshold exceeded: unattributed payments count (%d > 3) or sum (%d > 1000000 kopecks)", len(pendingAttributionPayments), pendingSumKopecks)
		}

		for _, p := range pendingAttributionPayments {
			kind := "deposit"
			if p.Purpose != "deposit" {
				kind = "gate"
			}
			var attr Attribution
			var linkID *string
			if *apply {
				err := cashxPool.QueryRow(ctx, `
					INSERT INTO external_user_attributions (project_id, partner_id, offer_id, external_user_id, first_seen_at)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (project_id, external_user_id) DO UPDATE SET partner_id = EXCLUDED.partner_id
					RETURNING id, partner_id, offer_id, tracking_link_id;
				`, projectID, *attrTo, defaultOfferID, p.UserID, p.UpdatedAt).Scan(&attr.ID, &attr.PartnerID, &attr.OfferID, &linkID)
				if err != nil {
					log.Fatalf("insert attribution for user %s: %v", p.UserID, err)
				}
				attr.TrackingLinkID = linkID
			} else {
				attr = Attribution{
					ID:        -1,
					PartnerID: *attrTo,
					OfferID:   defaultOfferID,
				}
			}
			attributedPayments = append(attributedPayments, attributedPayment{payment: p, attr: attr, kind: kind})
		}
	}

	report.Casino.AttributedCount = len(attributedPayments)
	for _, ap := range attributedPayments {
		report.Casino.AttributedSumKopecks += int64(ap.payment.AmountRub) * 100
	}
	report.Casino.UnattributedCount = len(report.Casino.Unattributed)
	for _, up := range report.Casino.Unattributed {
		report.Casino.UnattributedSum += up.AmountKopecks
	}

	// 5. Two-phase matching
	usedConversions := make(map[int64]bool)
	matchedPaymentIndexes := make(map[int]bool)

	// Phase 1: Exact matching by kazik-pay-<id>
	for i, ap := range attributedPayments {
		canonicalPayID := fmt.Sprintf("kazik-pay-%s", ap.payment.ID)
		expectedKopecks := int64(ap.payment.AmountRub) * 100
		for _, c := range conversions {
			if usedConversions[c.ID] {
				continue
			}
			if c.ExternalPaymentID == canonicalPayID {
				if c.AmountKopecks != expectedKopecks {
					report.Warnings = append(report.Warnings, fmt.Sprintf("CRITICAL ANOMALY: amount mismatch for payment %s: casino %d kopecks vs cashx %d kopecks", ap.payment.ID, expectedKopecks, c.AmountKopecks))
				}
				matchedPaymentIndexes[i] = true
				usedConversions[c.ID] = true
				report.Matching.MatchedExactCount++
				report.Matching.MatchedExactKopecks += c.AmountKopecks
				break
			}
		}
	}

	// Phase 2: Fallback matching by (external_user_id, amount_kopecks, occurred_at +- 2 min)
	type fallbackMatch struct {
		paymentIndex int
		convID       int64
	}
	var fallbackMatches []fallbackMatch

	for i, ap := range attributedPayments {
		if matchedPaymentIndexes[i] {
			continue
		}
		expectedKopecks := int64(ap.payment.AmountRub) * 100
		for _, c := range conversions {
			if usedConversions[c.ID] {
				continue
			}
			if c.ReversedAt != nil {
				continue // Active payments must not match already-reversed conversions
			}
			if c.ExternalUserID == ap.payment.UserID && c.AmountKopecks == expectedKopecks {
				diff := c.OccurredAt.Sub(ap.payment.UpdatedAt)
				if math.Abs(diff.Seconds()) <= 120 { // 2 minutes
					matchedPaymentIndexes[i] = true
					usedConversions[c.ID] = true
					report.Matching.MatchedFallbackCount++
					report.Matching.MatchedFallbackKop += c.AmountKopecks
					fallbackMatches = append(fallbackMatches, fallbackMatch{paymentIndex: i, convID: c.ID})
					break
				}
			}
		}
	}

	// Normalise fallback matches on -apply
	if *apply {
		for _, fm := range fallbackMatches {
			ap := attributedPayments[fm.paymentIndex]
			canonicalPayID := fmt.Sprintf("kazik-pay-%s", ap.payment.ID)
			canonicalEvtID := fmt.Sprintf("kazik-payment-%s", ap.payment.ID)

			// Check if key is already occupied by a DIFFERENT conversion
			var existingConvID pgtype.Int8
			err := cashxPool.QueryRow(ctx, `
				SELECT conversion_event_id FROM conversion_event_payments
				WHERE project_id = $1 AND external_payment_id = $2;
			`, projectID, canonicalPayID).Scan(&existingConvID)
			if err == nil && existingConvID.Valid && existingConvID.Int64 != fm.convID {
				// Mark duplicate orphan
				if _, err := cashxPool.Exec(ctx, `UPDATE conversion_events SET processing_note = 'reconcile:duplicate_orphan' WHERE id = $1`, fm.convID); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("mark duplicate_orphan %d: %v", fm.convID, err))
				}
				continue
			}

			// Update conversion event in place
			_, err = cashxPool.Exec(ctx, `
				UPDATE conversion_events
				SET external_payment_id = $1,
				    external_event_id = $2,
				    occurred_at = $3,
				    kind = $4
				WHERE id = $5;
			`, canonicalPayID, canonicalEvtID, ap.payment.UpdatedAt, ap.kind, fm.convID)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("update conversion %d: %v", fm.convID, err))
			}

			// Synchronize commission_earnings.created_at = conversion.occurred_at
			if _, err := cashxPool.Exec(ctx, `
				UPDATE commission_earnings
				SET created_at = $1
				WHERE conversion_event_id = $2;
			`, ap.payment.UpdatedAt, fm.convID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("update earning created_at for conv %d: %v", fm.convID, err))
			}

			// Upsert keys
			if _, err := cashxPool.Exec(ctx, `
				INSERT INTO conversion_event_payments (project_id, external_payment_id, conversion_event_id)
				VALUES ($1, $2, $3)
				ON CONFLICT (project_id, external_payment_id) DO UPDATE SET conversion_event_id = EXCLUDED.conversion_event_id;
			`, projectID, canonicalPayID, fm.convID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("upsert payment key %s: %v", canonicalPayID, err))
			}

			if _, err := cashxPool.Exec(ctx, `
				INSERT INTO incoming_event_keys (project_id, external_event_id)
				VALUES ($1, $2)
				ON CONFLICT (project_id, external_event_id) DO NOTHING;
			`, projectID, canonicalEvtID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert event key %s: %v", canonicalEvtID, err))
			}
		}
	}

	affectedWalletIDs := make(map[string]bool)

	// Missing payments
	var missingList []attributedPayment
	for i, ap := range attributedPayments {
		if !matchedPaymentIndexes[i] {
			missingList = append(missingList, ap)
			report.Matching.MissingCount++
			report.Matching.MissingKopecks += int64(ap.payment.AmountRub) * 100
			report.Matching.MissingDetails = append(report.Matching.MissingDetails, PaymentMovement{
				ID:            ap.payment.ID,
				UserID:        ap.payment.UserID,
				AmountKopecks: int64(ap.payment.AmountRub) * 100,
				Purpose:       ap.payment.Purpose,
				Kind:          ap.kind,
				UpdatedAt:     ap.payment.UpdatedAt,
			})
		}
	}

	if *apply && len(missingList) > 0 {
		for _, ap := range missingList {
			canonicalPayID := fmt.Sprintf("kazik-pay-%s", ap.payment.ID)
			canonicalEvtID := fmt.Sprintf("kazik-payment-%s", ap.payment.ID)
			amountKopecks := int64(ap.payment.AmountRub) * 100

			var newConvID int64
			err := cashxPool.QueryRow(ctx, `
				INSERT INTO conversion_events (project_id, attribution_id, external_user_id, external_payment_id, external_event_id, amount_kopecks, occurred_at, kind, processing_note)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'reconcile:missing')
				RETURNING id;
			`, projectID, ap.attr.ID, ap.payment.UserID, canonicalPayID, canonicalEvtID, amountKopecks, ap.payment.UpdatedAt, ap.kind).Scan(&newConvID)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert missing conversion %s: %v", ap.payment.ID, err))
				continue
			}

			if _, err := cashxPool.Exec(ctx, `
				INSERT INTO conversion_event_payments (project_id, external_payment_id, conversion_event_id)
				VALUES ($1, $2, $3)
				ON CONFLICT (project_id, external_payment_id) DO UPDATE SET conversion_event_id = EXCLUDED.conversion_event_id;
			`, projectID, canonicalPayID, newConvID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert payment key for missing conv %s: %v", canonicalPayID, err))
			}

			if _, err := cashxPool.Exec(ctx, `
				INSERT INTO incoming_event_keys (project_id, external_event_id)
				VALUES ($1, $2)
				ON CONFLICT (project_id, external_event_id) DO NOTHING;
			`, projectID, canonicalEvtID); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert event key for missing conv %s: %v", canonicalEvtID, err))
			}

			// Compute partner commission
			var rateBps int
			err = cashxPool.QueryRow(ctx, `
				SELECT rate_bps FROM partner_offer_accesses WHERE partner_id = $1 AND offer_id = $2 AND status = 'active';
			`, ap.attr.PartnerID, ap.attr.OfferID).Scan(&rateBps)
			if err != nil {
				_ = cashxPool.QueryRow(ctx, `
					SELECT revshare_percent_bps FROM partner_profiles WHERE id = $1;
				`, ap.attr.PartnerID).Scan(&rateBps)
			}
			earningAmount := amountKopecks * int64(rateBps) / 10000

			var earningID string
			err = cashxPool.QueryRow(ctx, `
				INSERT INTO commission_earnings (conversion_event_id, partner_id, offer_id, tracking_link_id, rate_bps, amount_kopecks, external_user_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id;
			`, newConvID, ap.attr.PartnerID, ap.attr.OfferID, ap.attr.TrackingLinkID, rateBps, earningAmount, ap.payment.UserID, ap.payment.UpdatedAt).Scan(&earningID)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert earning for missing conversion %d: %v", newConvID, err))
				continue
			}

			// Add ledger entry
			if earningAmount > 0 {
				var walletID string
				if err := cashxPool.QueryRow(ctx, "SELECT id FROM wallets WHERE partner_id = $1", ap.attr.PartnerID).Scan(&walletID); err == nil {
					affectedWalletIDs[walletID] = true
					if _, err := cashxPool.Exec(ctx, `
						INSERT INTO wallet_ledger_entries (wallet_id, type, amount_kopecks, balance_after_kopecks, ref_conversion_event_id, comment)
						VALUES ($1, 'commission', $2, (SELECT COALESCE(SUM(amount_kopecks), 0) + $2 FROM wallet_ledger_entries WHERE wallet_id = $1), $3, 'reconcile: missing conversion credit');
					`, walletID, earningAmount, newConvID); err != nil {
						report.Errors = append(report.Errors, fmt.Sprintf("insert ledger entry for missing conv %d: %v", newConvID, err))
					}
				}
			}
		}
	}

	// Orphan conversions (strictly in window [from, to))
	for _, c := range conversions {
		if !usedConversions[c.ID] {
			var earningID, partnerID string
			var earningAmount int64
			var earningReversedAt *time.Time
			err := cashxPool.QueryRow(ctx, `
				SELECT id, partner_id, amount_kopecks, reversed_at
				FROM commission_earnings WHERE conversion_event_id = $1;
			`, c.ID).Scan(&earningID, &partnerID, &earningAmount, &earningReversedAt)
			earningUnreversed := (err == nil && earningReversedAt == nil)

			if c.ReversedAt != nil && !earningUnreversed {
				// Already fully stornoed/reversed in a prior run or refund; skip counting as new orphan.
				continue
			}

			report.Matching.OrphanCount++
			report.Matching.OrphanKopecks += c.AmountKopecks
			report.Matching.OrphanDetails = append(report.Matching.OrphanDetails, c)

			if *apply {
				// Mark reconcile:orphan
				if _, err := cashxPool.Exec(ctx, `UPDATE conversion_events SET processing_note = 'reconcile:orphan' WHERE id = $1`, c.ID); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("mark reconcile:orphan %d: %v", c.ID, err))
				}

				// Idempotent storno
				if c.ReversedAt == nil {
					if _, err := cashxPool.Exec(ctx, `UPDATE conversion_events SET reversed_at = now() WHERE id = $1 AND reversed_at IS NULL`, c.ID); err != nil {
						report.Errors = append(report.Errors, fmt.Sprintf("set conversion %d reversed: %v", c.ID, err))
					}
				}

				// Storno commission_earnings
				if earningUnreversed {
					if _, err := cashxPool.Exec(ctx, `UPDATE commission_earnings SET reversed_at = now() WHERE id = $1 AND reversed_at IS NULL`, earningID); err != nil {
						report.Errors = append(report.Errors, fmt.Sprintf("set earning %s reversed: %v", earningID, err))
					}

					if earningAmount > 0 {
						var walletID string
						if err := cashxPool.QueryRow(ctx, "SELECT id FROM wallets WHERE partner_id = $1", partnerID).Scan(&walletID); err == nil {
							affectedWalletIDs[walletID] = true
							if _, err := cashxPool.Exec(ctx, `
								INSERT INTO wallet_ledger_entries (wallet_id, type, amount_kopecks, balance_after_kopecks, ref_conversion_event_id, comment)
								VALUES ($1, 'reversal', -$2, (SELECT COALESCE(SUM(amount_kopecks), 0) - $2 FROM wallet_ledger_entries WHERE wallet_id = $1), $3, 'reconcile: orphan conversion reversed');
							`, walletID, earningAmount, c.ID); err != nil {
								report.Errors = append(report.Errors, fmt.Sprintf("insert ledger reversal for orphan %d: %v", c.ID, err))
							}
						}
					}

					// Storno referral_rewards if any
					var rewardID, referrerPartnerID string
					var rewardAmount int64
					var rewardReversedAt *time.Time
					err := cashxPool.QueryRow(ctx, `
						SELECT id, referrer_partner_id, amount_kopecks, reversed_at
						FROM referral_rewards WHERE commission_earning_id = $1;
					`, earningID).Scan(&rewardID, &referrerPartnerID, &rewardAmount, &rewardReversedAt)
					if err == nil && rewardReversedAt == nil {
						if _, err := cashxPool.Exec(ctx, `UPDATE referral_rewards SET reversed_at = now() WHERE id = $1 AND reversed_at IS NULL`, rewardID); err != nil {
							report.Errors = append(report.Errors, fmt.Sprintf("set referral reward %s reversed: %v", rewardID, err))
						}
						if rewardAmount > 0 {
							var refWalletID string
							if err := cashxPool.QueryRow(ctx, "SELECT id FROM wallets WHERE partner_id = $1", referrerPartnerID).Scan(&refWalletID); err == nil {
								affectedWalletIDs[refWalletID] = true
								if _, err := cashxPool.Exec(ctx, `
									INSERT INTO wallet_ledger_entries (wallet_id, type, amount_kopecks, balance_after_kopecks, ref_referral_reward_id, comment)
									VALUES ($1, 'reversal', -$2, (SELECT COALESCE(SUM(amount_kopecks), 0) - $2 FROM wallet_ledger_entries WHERE wallet_id = $1), $3, 'reconcile: orphan referral reward reversed');
								`, refWalletID, rewardAmount, rewardID); err != nil {
									report.Errors = append(report.Errors, fmt.Sprintf("insert referral ledger reversal for reward %s: %v", rewardID, err))
								}
							}
						}
					}
				}
			}
		}
	}

	// 6. Resync all affected wallets
	if *apply && len(affectedWalletIDs) > 0 {
		for wID := range affectedWalletIDs {
			var beforeAvail, reserved int64
			var partnerID string
			_ = cashxPool.QueryRow(ctx, "SELECT partner_id, available_kopecks, reserved_kopecks FROM wallets WHERE id = $1", wID).Scan(&partnerID, &beforeAvail, &reserved)

			var afterAvail int64
			err := cashxPool.QueryRow(ctx, `
				UPDATE wallets w
				SET available_kopecks = (
					COALESCE((SELECT SUM(amount_kopecks) FROM wallet_ledger_entries WHERE wallet_id = w.id), 0)
				),
				updated_at = now()
				WHERE id = $1
				RETURNING available_kopecks;
			`, wID).Scan(&afterAvail)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("resync wallet %s: %v", wID, err))
			} else {
				report.Wallets = append(report.Wallets, WalletChange{
					WalletID:        wID,
					PartnerID:       partnerID,
					BeforeAvailable: beforeAvail,
					AfterAvailable:  afterAvail,
					ReservedKopecks: reserved,
				})
			}
		}
	}

	// 7. Recompute daily stats on -apply
	if *apply {
		fmt.Printf("Recomputing daily stats from %s to %s (tomorrow MSK)...\n", from.Format("2006-01-02"), tomorrowMSK.Format("2006-01-02"))
		if err := tracking.RecomputeDailyStats(ctx, cashxPool, from, tomorrowMSK); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("recompute daily stats: %v", err))
		}
	}

	// 8. Integrity check & Final summary queries
	var activeConvCount, activeConvSum int64
	_ = cashxPool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(amount_kopecks), 0)
		FROM conversion_events
		WHERE project_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		  AND reversed_at IS NULL;
	`, projectID, from, to).Scan(&activeConvCount, &activeConvSum)

	report.Cashx.ActiveConversionsCount = activeConvCount
	report.Cashx.ActiveSumKopecks = activeConvSum
	report.Cashx.ActiveSumRubles = float64(activeConvSum) / 100.0

	var activeEarningsSum int64
	_ = cashxPool.QueryRow(ctx, `
		SELECT COALESCE(sum(ce.amount_kopecks), 0)
		FROM commission_earnings ce
		JOIN conversion_events c ON c.id = ce.conversion_event_id
		WHERE c.project_id = $1 AND c.occurred_at >= $2 AND c.occurred_at < $3
		  AND ce.reversed_at IS NULL;
	`, projectID, from, to).Scan(&activeEarningsSum)

	var statsFP, statsIncome int64
	_ = cashxPool.QueryRow(ctx, `
		SELECT COALESCE(sum(first_payments), 0), COALESCE(sum(income_kopecks), 0)
		FROM daily_partner_offer_stats
		WHERE day >= ($1 AT TIME ZONE 'Europe/Moscow')::date AND day < ($2 AT TIME ZONE 'Europe/Moscow')::date;
	`, from, to).Scan(&statsFP, &statsIncome)

	passed := true
	if *apply {
		if statsFP != activeConvCount {
			passed = false
			report.Warnings = append(report.Warnings, fmt.Sprintf("stats_first_payments (%d) != active_conversions_count (%d)", statsFP, activeConvCount))
		}
		if statsIncome != activeEarningsSum {
			passed = false
			report.Warnings = append(report.Warnings, fmt.Sprintf("stats_income_kopecks (%d) != active_earnings_sum (%d)", statsIncome, activeEarningsSum))
		}
	}

	report.Integrity = IntegrityCheck{
		ActiveConversionsCount: activeConvCount,
		ActiveConversionsSum:   activeConvSum,
		ActiveEarningsSum:      activeEarningsSum,
		StatsFirstPayments:     statsFP,
		StatsIncomeKopecks:     statsIncome,
		Passed:                 passed,
	}

	// Print summary
	fmt.Printf("\n--- Reconcile Summary ---\n")
	fmt.Printf("Casino Payments:   %d (%.2f ₽)\n", report.Casino.TotalPayments, report.Casino.TotalAmountRubles)
	fmt.Printf("  Attributed:      %d (%.2f ₽)\n", report.Casino.AttributedCount, float64(report.Casino.AttributedSumKopecks)/100.0)
	fmt.Printf("  Unattributed:    %d (%.2f ₽)\n", report.Casino.UnattributedCount, float64(report.Casino.UnattributedSum)/100.0)
	fmt.Printf("Cashx Conversions: %d initial -> %d active (%.2f ₽)\n", report.Cashx.InitialConversionsCount, report.Cashx.ActiveConversionsCount, report.Cashx.ActiveSumRubles)
	fmt.Printf("Matching:\n")
	fmt.Printf("  Exact matched:    %d (%.2f ₽)\n", report.Matching.MatchedExactCount, float64(report.Matching.MatchedExactKopecks)/100.0)
	fmt.Printf("  Fallback matched: %d (%.2f ₽)\n", report.Matching.MatchedFallbackCount, float64(report.Matching.MatchedFallbackKop)/100.0)
	fmt.Printf("  Missing:          %d (%.2f ₽)\n", report.Matching.MissingCount, float64(report.Matching.MissingKopecks)/100.0)
	fmt.Printf("  Orphans:          %d (%.2f ₽)\n", report.Matching.OrphanCount, float64(report.Matching.OrphanKopecks)/100.0)
	if len(report.Wallets) > 0 {
		fmt.Printf("Wallets resynced:\n")
		for _, w := range report.Wallets {
			fmt.Printf("  Wallet %s (partner %s): %.2f ₽ -> %.2f ₽ (reserved: %.2f ₽)\n",
				w.WalletID, w.PartnerID, float64(w.BeforeAvailable)/100.0, float64(w.AfterAvailable)/100.0, float64(w.ReservedKopecks)/100.0)
		}
	}
	if len(report.FraudAudit) > 0 {
		fmt.Printf("Fraud audit entries: %d\n", len(report.FraudAudit))
		for _, f := range report.FraudAudit {
			fmt.Printf("  User %s: %d events (%.2f ₽)\n", f.ExternalUserID, f.EventsCount, float64(f.TotalKopecks)/100.0)
		}
	}
	fmt.Printf("Integrity Passed: %v\n", report.Integrity.Passed)

	// Write report
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(*reportPath, data, 0644); err != nil {
		log.Fatalf("write report to %s: %v", *reportPath, err)
	}
	fmt.Printf("Report written to %s\n", *reportPath)
}
