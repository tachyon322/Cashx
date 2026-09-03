package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/notifications"
	"cashx/internal/platform"
	"cashx/internal/repository"
	"cashx/internal/tracking"
)

// EventInput is the signed event body sent by a project.
type EventInput struct {
	EventID           string     `json:"event_id"`
	Type              string     `json:"type"`
	OccurredAt        time.Time  `json:"occurred_at"`
	ExternalUserID    string     `json:"external_user_id"`
	ClickToken        *string    `json:"click_token"`
	SourceCode        *string    `json:"source_code"`
	ExternalPaymentID *string    `json:"external_payment_id"`
	AmountKopecks     *int64     `json:"amount_kopecks"`
	Currency          *string    `json:"currency"`
}

// EventSource describes the traffic source an event was attributed to.
type EventSource struct {
	Code             string `json:"code"`
	Type             string `json:"type"`
	IsPromo          bool   `json:"is_promo"`
	RegistrationBonus *int32 `json:"registration_bonus,omitempty"`
}

// ProcessResult is the API response for an event.
type ProcessResult struct {
	Status string      `json:"status"` // accepted | duplicate | ignored
	Reason string      `json:"reason,omitempty"`
	Source *EventSource `json:"source,omitempty"`
}

// Service processes project events.
type Service struct {
	Pool             *pgxpool.Pool
	ClickTokenSecret string
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// Process handles one event in a single transaction.
func (s *Service) Process(ctx context.Context, projectID string, rawBody []byte) (ProcessResult, error) {
	var in EventInput
	if err := json.Unmarshal(rawBody, &in); err != nil {
		return ProcessResult{}, fmt.Errorf("%w: invalid_payload", platform.ErrValidation)
	}
	if in.EventID == "" || in.Type == "" || in.ExternalUserID == "" || in.OccurredAt.IsZero() {
		return ProcessResult{}, fmt.Errorf("%w: invalid_payload", platform.ErrValidation)
	}
	if in.Type != "registration.created" && in.Type != "revenue.confirmed" && in.Type != "revenue.reversed" {
		return ProcessResult{}, fmt.Errorf("%w: invalid_payload", platform.ErrValidation)
	}

	// Ensure the monthly partitions for the event's month (and the current
	// one) exist before any insert — best-effort guard against SQLSTATE 23514
	// (see migrations/00021_partition_security.sql).
	_ = tracking.EnsurePartitionsFor(ctx, s.Pool, in.OccurredAt)

	var result ProcessResult
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		// Idempotency: serialize concurrent replays of the same event and
		// check the incoming log before doing any work.
		if err := tq.EventLock(ctx, projectID+":"+in.EventID); err != nil {
			return err
		}
		if _, err := tq.GetIncomingEvent(ctx, repository.GetIncomingEventParams{ProjectID: projectID, ExternalEventID: in.EventID}); err == nil {
			result = ProcessResult{Status: "duplicate"}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		var err error
		switch in.Type {
		case "registration.created":
			result, err = s.processRegistration(ctx, tq, projectID, &in)
		case "revenue.confirmed":
			result, err = s.processConfirmed(ctx, tq, projectID, &in)
		case "revenue.reversed":
			result, err = s.processReversed(ctx, tq, projectID, &in)
		}
		if err != nil {
			return err
		}

		status := "processed"
		reason := (*string)(nil)
		if result.Status == "ignored" {
			status = "ignored"
			reason = &result.Reason
		}
		_, err = tq.InsertIncomingEvent(ctx, repository.InsertIncomingEventParams{
			ProjectID: projectID, ExternalEventID: in.EventID, Type: in.Type,
			Payload: rawBody, Status: status, Reason: repository.TextPtr(reason),
		})
		return err
	})
	return result, err
}

func (s *Service) processRegistration(ctx context.Context, tq *repository.Queries, projectID string, in *EventInput) (ProcessResult, error) {
	// Attribution path 1 (preferred): a signed click_token from the redirect
	// service (/c/{code}) captured by the project frontend at registration.
	if in.ClickToken != nil && *in.ClickToken != "" {
		clickID, err := tracking.VerifyClickToken(s.ClickTokenSecret, *in.ClickToken)
		if err != nil {
			// Fall through to the source_code path — the token may have expired.
			if in.SourceCode == nil || *in.SourceCode == "" {
				return ProcessResult{Status: "ignored", Reason: "invalid_click_token"}, nil
			}
		} else {
			click, err := tq.GetClickWithLink(ctx, clickID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					if in.SourceCode == nil || *in.SourceCode == "" {
						return ProcessResult{Status: "ignored", Reason: "invalid_click_token"}, nil
					}
				} else {
					return ProcessResult{}, err
				}
			} else {
				// First-touch: existing attribution is never overwritten.
				_, err = tq.CreateAttribution(ctx, repository.CreateAttributionParams{
					ProjectID:       projectID,
					TrackingClickID: pgtype.Int8{Int64: clickID, Valid: true},
					TrackingLinkID:  repository.UUIDPtr(&click.TrackingLinkID),
					PartnerID:       repository.UUIDPtr(&click.PartnerID),
					OfferID:         repository.UUIDPtr(&click.OfferID),
					ExternalUserID:  in.ExternalUserID,
				})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return ProcessResult{}, err
				}
				return ProcessResult{Status: "accepted"}, nil
			}
		}
	}

	// Attribution path 2: a source/promo code (e.g. the user registered with
	// ?ref=PROMOCODE). No click exists for promo sources, so the attribution
	// is tied to the tracking link itself.
	if in.SourceCode == nil || *in.SourceCode == "" {
		return ProcessResult{Status: "ignored", Reason: "invalid_click_token"}, nil
	}
	code := strings.ToUpper(strings.TrimSpace(*in.SourceCode))
	link, err := tq.GetTrackingLinkByCodeForProject(ctx, repository.GetTrackingLinkByCodeForProjectParams{
		Code: code, ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProcessResult{Status: "ignored", Reason: "unknown_source_code"}, nil
		}
		return ProcessResult{}, err
	}
	if !link.IsActive || link.AccessStatus != "active" {
		return ProcessResult{Status: "ignored", Reason: "inactive_source"}, nil
	}
	// First-touch: existing attribution is never overwritten.
	_, err = tq.CreateAttribution(ctx, repository.CreateAttributionParams{
		ProjectID:      projectID,
		PartnerID:      repository.UUIDPtr(&link.PartnerID),
		OfferID:        repository.UUIDPtr(&link.OfferID),
		TrackingLinkID: repository.UUIDPtr(&link.ID),
		ExternalUserID: in.ExternalUserID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Status: "accepted",
		Source: &EventSource{
			Code:              link.Code,
			Type:              link.Type,
			IsPromo:           link.Type == "promo",
			RegistrationBonus: bonusPtr(link.RegistrationBonus),
		},
	}, nil
}

func bonusPtr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func (s *Service) processConfirmed(ctx context.Context, tq *repository.Queries, projectID string, in *EventInput) (ProcessResult, error) {
	currency := "RUB"
	if in.Currency != nil && *in.Currency != "" {
		currency = *in.Currency
	}
	if currency != "RUB" {
		return ProcessResult{Status: "ignored", Reason: "unsupported_currency"}, nil
	}
	if in.AmountKopecks == nil || *in.AmountKopecks <= 0 {
		return ProcessResult{Status: "ignored", Reason: "invalid_amount"}, nil
	}
	if in.ExternalPaymentID == nil || *in.ExternalPaymentID == "" {
		return ProcessResult{Status: "ignored", Reason: "invalid_payment_id"}, nil
	}
	attr, err := tq.GetAttributionByProjectUser(ctx, repository.GetAttributionByProjectUserParams{
		ProjectID: projectID, ExternalUserID: in.ExternalUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProcessResult{Status: "ignored", Reason: "no_attribution"}, nil
		}
		return ProcessResult{}, err
	}
	attrPartnerID := *repository.UUIDToPtr(attr.PartnerID)
	attrOfferID := *repository.UUIDToPtr(attr.OfferID)
	// Payment/event idempotency.
	if _, err := tq.GetConversionByEvent(ctx, repository.GetConversionByEventParams{ProjectID: projectID, ExternalEventID: in.EventID}); err == nil {
		return ProcessResult{Status: "duplicate"}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ProcessResult{}, err
	}
	if _, err := tq.GetConversionByPayment(ctx, repository.GetConversionByPaymentParams{ProjectID: projectID, ExternalPaymentID: *in.ExternalPaymentID}); err == nil {
		return ProcessResult{Status: "duplicate"}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ProcessResult{}, err
	}

	conv, err := tq.InsertConversion(ctx, repository.InsertConversionParams{
		ProjectID: projectID, ExternalEventID: in.EventID, ExternalPaymentID: *in.ExternalPaymentID,
		ExternalUserID: in.ExternalUserID, AttributionID: attr.ID,
		AmountKopecks: *in.AmountKopecks, Currency: currency,
		OccurredAt: repository.TimePtr(&in.OccurredAt),
	})
	if err != nil {
		return ProcessResult{}, err
	}

	// Commission at the partner's personal offer rate (snapshot in the earning).
	access, err := tq.GetPartnerAccess(ctx, repository.GetPartnerAccessParams{PartnerID: attrPartnerID, OfferID: attrOfferID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ProcessResult{}, err
	}
	if err != nil || access.Status != "active" || access.RateBps <= 0 {
		note := "no_active_access"
		if err := tq.UpdateConversionNote(ctx, repository.UpdateConversionNoteParams{ID: conv.ID, ProcessingNote: repository.TextPtr(&note)}); err != nil {
			return ProcessResult{}, err
		}
		return ProcessResult{Status: "accepted"}, nil
	}
	earning := *in.AmountKopecks * int64(access.RateBps) / 10000
	if earning <= 0 {
		return ProcessResult{Status: "accepted"}, nil
	}
	// Attribute the earning to the traffic source (tracking link) of the
	// first-touch attribution. Promo attributions carry the link directly;
	// click-based attributions resolve it through the click.
	linkID := pgtype.UUID{}
	if attr.TrackingLinkID.Valid {
		linkID = attr.TrackingLinkID
	} else if attr.TrackingClickID.Valid {
		if click, err := tq.GetClickWithLink(ctx, attr.TrackingClickID.Int64); err == nil {
			linkID = repository.UUIDPtr(&click.TrackingLinkID)
		}
	}
	earningRow, err := tq.InsertCommissionEarning(ctx, repository.InsertCommissionEarningParams{
		ConversionEventID: conv.ID, PartnerID: attrPartnerID, OfferID: attrOfferID,
		TrackingLinkID: linkID, RateBps: access.RateBps,
		AmountKopecks: earning, ExternalUserID: in.ExternalUserID,
	})
	if err != nil {
		return ProcessResult{}, err
	}
	if err := s.credit(ctx, tq, attrPartnerID, "commission", earning, conv.ID, nil, nil,
		"Начислена комиссия", "Комиссия по офферу"); err != nil {
		return ProcessResult{}, err
	}

	// Referral reward: 2.5% of the actually accrued commission.
	profile, err := tq.GetPartnerProfileByID(ctx, attrPartnerID)
	if err != nil {
		return ProcessResult{}, err
	}
	if profile.ReferredBy.Valid {
		ref, err := tq.GetPartnerReferralByInvited(ctx, attrPartnerID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return ProcessResult{}, err
		}
		if err == nil {
			reward := earning * int64(ref.ReferralRateBps) / 10000
			if reward > 0 {
				rewardRow, err := tq.InsertReferralReward(ctx, repository.InsertReferralRewardParams{
					CommissionEarningID: earningRow.ID, PartnerReferralID: ref.ID,
					ReferrerPartnerID: ref.ReferrerPartnerID, InvitedPartnerID: attrPartnerID,
					ReferralRateBps: ref.ReferralRateBps, AmountKopecks: reward,
				})
				if err != nil {
					return ProcessResult{}, err
				}
				if err := s.credit(ctx, tq, ref.ReferrerPartnerID, "referral_reward", reward, 0, nil, &rewardRow.ID,
					"Реферальное вознаграждение", "Вознаграждение за партнёра"); err != nil {
					return ProcessResult{}, err
				}
			}
		}
	}
	return ProcessResult{Status: "accepted"}, nil
}

func (s *Service) processReversed(ctx context.Context, tq *repository.Queries, projectID string, in *EventInput) (ProcessResult, error) {
	if in.ExternalPaymentID == nil || *in.ExternalPaymentID == "" {
		return ProcessResult{Status: "ignored", Reason: "invalid_payment_id"}, nil
	}
	conv, err := tq.GetConversionByPayment(ctx, repository.GetConversionByPaymentParams{
		ProjectID: projectID, ExternalPaymentID: *in.ExternalPaymentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProcessResult{Status: "ignored", Reason: "payment_not_found"}, nil
		}
		return ProcessResult{}, err
	}
	earning, err := tq.GetEarningByConversion(ctx, conv.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Nothing accrued for this conversion — nothing to reverse.
			return ProcessResult{Status: "accepted"}, nil
		}
		return ProcessResult{}, err
	}
	reversed, err := tq.ReverseEarning(ctx, earning.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProcessResult{Status: "duplicate"}, nil
		}
		return ProcessResult{}, err
	}
	if err := s.debit(ctx, tq, earning.PartnerID, "reversal", reversed.AmountKopecks, conv.ID, nil, nil,
		"Платёж отменён, комиссия списана"); err != nil {
		return ProcessResult{}, err
	}

	// Reverse the referrer reward if one was granted on this earning.
	if reward, err := tq.GetRewardByEarning(ctx, earning.ID); err == nil {
		rev, err := tq.ReverseReward(ctx, reward.ID)
		if err == nil && rev.AmountKopecks > 0 {
			if err := s.debit(ctx, tq, reward.ReferrerPartnerID, "reversal", rev.AmountKopecks, 0, nil, &rev.ID,
				"Платёж отменён, вознаграждение списано"); err != nil {
				return ProcessResult{}, err
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ProcessResult{}, err
	}
	return ProcessResult{Status: "accepted"}, nil
}

// credit adds money to a partner's wallet with a ledger entry and notification.
func (s *Service) credit(ctx context.Context, tq *repository.Queries, partnerID, entryType string, amount int64, convID int64, withdrawalID *string, rewardID *string, title, body string) error {
	wallet, err := tq.GetWalletByPartnerID(ctx, partnerID)
	if err != nil {
		return err
	}
	updated, err := tq.CreditWallet(ctx, repository.CreditWalletParams{ID: wallet.ID, AvailableKopecks: amount})
	if err != nil {
		return err
	}
	if _, err := tq.InsertLedgerEntry(ctx, repository.InsertLedgerEntryParams{
		WalletID: wallet.ID, Type: entryType, AmountKopecks: amount,
		BalanceAfterKopecks: updated.AvailableKopecks,
		RefConversionEventID: pgtypeInt8(convID),
		RefWithdrawalID:      uuidPtr(withdrawalID),
		RefReferralRewardID:  uuidPtr(rewardID),
	}); err != nil {
		return err
	}
	userID, err := tq.GetUserIDByPartnerID(ctx, partnerID)
	if err != nil {
		return err
	}
	return notifications.NotifyUser(ctx, tq, userID, entryType, title, body, nil)
}

// debit subtracts money (available may go negative — debt) with a ledger entry.
func (s *Service) debit(ctx context.Context, tq *repository.Queries, partnerID, entryType string, amount int64, convID int64, withdrawalID *string, rewardID *string, body string) error {
	wallet, err := tq.GetWalletByPartnerID(ctx, partnerID)
	if err != nil {
		return err
	}
	updated, err := tq.CreditWallet(ctx, repository.CreditWalletParams{ID: wallet.ID, AvailableKopecks: -amount})
	if err != nil {
		return err
	}
	if _, err := tq.InsertLedgerEntry(ctx, repository.InsertLedgerEntryParams{
		WalletID: wallet.ID, Type: entryType, AmountKopecks: -amount,
		BalanceAfterKopecks: updated.AvailableKopecks,
		RefConversionEventID: pgtypeInt8(convID),
		RefWithdrawalID:      uuidPtr(withdrawalID),
		RefReferralRewardID:  uuidPtr(rewardID),
		Comment:              repository.TextPtr(&body),
	}); err != nil {
		return err
	}
	userID, err := tq.GetUserIDByPartnerID(ctx, partnerID)
	if err != nil {
		return err
	}
	return notifications.NotifyUser(ctx, tq, userID, "withdrawal", "Платёж отменён", body, nil)
}

func pgtypeInt8(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func uuidPtr(s *string) pgtype.UUID {
	return repository.UUIDPtr(s)
}
