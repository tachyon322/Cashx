// Package payouts implements withdrawal requests and payout rules.
package payouts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/audit"
	"cashx/internal/notifications"
	"cashx/internal/platform"
	"cashx/internal/repository"
)

// Config is the payout rules singleton.
type Config struct {
	MinWithdrawKopecks  int64   `json:"min_withdraw_kopecks"`
	UsdtRate            float64 `json:"usdt_rate"`
	SbpFeeFlatKopecks   int64   `json:"sbp_fee_flat_kopecks"`
	SbpFeePercentBps    int     `json:"sbp_fee_percent_bps"`
	UpdatedAt           string  `json:"updated_at"`
}

// Service implements payout operations.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.Recorder
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// GetConfig returns the payout rules.
func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	row, err := s.q(ctx).GetPayoutRules(ctx)
	if err != nil {
		return Config{}, err
	}
	return rowConfig(row), nil
}

func rowConfig(row repository.PayoutRule) Config {
	return Config{
		MinWithdrawKopecks: row.MinWithdrawKopecks,
		UsdtRate:           repository.NumToFloat64(row.UsdtRate),
		SbpFeeFlatKopecks:  row.SbpFeeFlatKopecks,
		SbpFeePercentBps:   int(row.SbpFeePercentBps),
		UpdatedAt:          row.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
}

// UpdateRules patches the payout rules (all fields required).
func (s *Service) UpdateRules(ctx context.Context, actorID *string, min int64, rate float64, flat int64, percentBps int) (Config, error) {
	if min <= 0 || rate <= 0 || percentBps < 0 || percentBps > 10000 {
		return Config{}, fmt.Errorf("%w: invalid_rules", platform.ErrValidation)
	}
	row, err := s.q(ctx).UpdatePayoutRules(ctx, repository.UpdatePayoutRulesParams{
		MinWithdrawKopecks: min, UsdtRate: repository.Float64ToNum(&rate),
		SbpFeeFlatKopecks: flat, SbpFeePercentBps: int32(percentBps),
		UpdatedBy: repository.UUIDPtr(actorID),
	})
	if err != nil {
		return Config{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "payout_rules.updated", "payout_rules", "singleton",
			map[string]any{"min_withdraw_kopecks": min, "usdt_rate": rate, "sbp_fee_flat_kopecks": flat, "sbp_fee_percent_bps": percentBps}, nil)
	}
	return rowConfig(row), nil
}

// Request creates a withdrawal request, atomically reserving the amount.
func (s *Service) Request(ctx context.Context, partnerID, method string, amountKopecks int64, requisites, bank string) (repository.WithdrawalRequest, error) {
	method = string(method)
	if method != "usdt" && method != "sbp" {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: invalid_method", platform.ErrValidation)
	}
	if amountKopecks <= 0 {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: invalid_amount", platform.ErrValidation)
	}
	requisites = trim(requisites)
	if requisites == "" {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: invalid_requisites", platform.ErrValidation)
	}
	if method == "sbp" && trim(bank) == "" {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: bank_required", platform.ErrValidation)
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return repository.WithdrawalRequest{}, err
	}
	if amountKopecks < cfg.MinWithdrawKopecks {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: below_min_withdraw", platform.ErrValidation)
	}
	if method == "usdt" && cfg.UsdtRate <= 0 {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
	}

	var fee int64
	var usdtAmount *float64
	var rate *float64
	if method == "sbp" {
		fee = cfg.SbpFeeFlatKopecks + amountKopecks*int64(cfg.SbpFeePercentBps)/10000
	} else {
		usd := round8(float64(amountKopecks) / cfg.UsdtRate)
		usdtAmount = &usd
		rate = &cfg.UsdtRate
	}

	var out repository.WithdrawalRequest
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		wallet, err := tq.GetWalletByPartnerID(ctx, partnerID)
		if err != nil {
			return err
		}
		// Atomic guard: two parallel requests cannot exceed available.
		updated, err := tq.ReserveWithdrawal(ctx, repository.ReserveWithdrawalParams{
			ID: wallet.ID, AvailableKopecks: amountKopecks,
		})
		if err != nil {
			return err
		}
		if updated.AvailableKopecks < 0 || updated.ReservedKopecks < amountKopecks {
			return fmt.Errorf("%w: insufficient_balance", platform.ErrConflict)
		}
		row, err := tq.InsertWithdrawalRequest(ctx, repository.InsertWithdrawalRequestParams{
			PartnerID: partnerID, AmountKopecks: amountKopecks, Method: method,
			Requisites: requisites, Bank: repository.TextPtr(nilIfEmpty(bank)),
			FeeKopecks: fee, UsdtAmount: repository.Float64ToNum(usdtAmount), Rate: repository.Float64ToNum(rate),
		})
		if err != nil {
			return err
		}
		comment := "Вывод средств"
		if _, err := tq.InsertLedgerEntry(ctx, repository.InsertLedgerEntryParams{
			WalletID: wallet.ID, Type: "withdrawal", AmountKopecks: -amountKopecks,
			BalanceAfterKopecks: updated.AvailableKopecks,
			RefWithdrawalID:     repository.UUIDPtr(&row.ID),
			Comment:             repository.TextPtr(&comment),
		}); err != nil {
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.WithdrawalRequest{}, fmt.Errorf("%w: insufficient_balance", platform.ErrConflict)
		}
		return repository.WithdrawalRequest{}, err
	}
	return out, nil
}

// Cancel cancels an own pending request, releasing the reserve.
func (s *Service) Cancel(ctx context.Context, partnerID, id string) (repository.WithdrawalRequest, error) {
	var out repository.WithdrawalRequest
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		row, err := tq.GetWithdrawalRequest(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: withdrawal_not_found", platform.ErrNotFound)
			}
			return err
		}
		if row.PartnerID != partnerID {
			return fmt.Errorf("%w: withdrawal_not_found", platform.ErrNotFound)
		}
		if row.Status != "pending" {
			return fmt.Errorf("%w: withdrawal_not_pending", platform.ErrConflict)
		}
		claimed, err := tq.DecideWithdrawal(ctx, repository.DecideWithdrawalParams{
			ID: id, Status: "cancelled", Comment: repository.TextPtr(nil),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: withdrawal_not_pending", platform.ErrConflict)
			}
			return err
		}
		wallet, err := tq.GetWalletByPartnerID(ctx, partnerID)
		if err != nil {
			return err
		}
		updated, err := tq.ReleaseReserve(ctx, repository.ReleaseReserveParams{ID: wallet.ID, AvailableKopecks: row.AmountKopecks})
		if err != nil {
			return err
		}
		comment := "Отмена заявки"
		if _, err := tq.InsertLedgerEntry(ctx, repository.InsertLedgerEntryParams{
			WalletID: wallet.ID, Type: "withdrawal_refund", AmountKopecks: row.AmountKopecks,
			BalanceAfterKopecks: updated.AvailableKopecks,
			RefWithdrawalID:     repository.UUIDPtr(&row.ID),
			Comment:             repository.TextPtr(&comment),
		}); err != nil {
			return err
		}
		out = claimed
		return nil
	})
	if err != nil {
		return repository.WithdrawalRequest{}, err
	}
	return out, nil
}

// Decide approves or rejects a pending request. Rejection returns the reserve.
func (s *Service) Decide(ctx context.Context, actorID *string, id, decision, comment string) (repository.WithdrawalRequest, error) {
	if decision != "approved" && decision != "rejected" {
		return repository.WithdrawalRequest{}, fmt.Errorf("%w: invalid_decision", platform.ErrValidation)
	}
	var out repository.WithdrawalRequest
	var userID string
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		claimed, err := tq.DecideWithdrawal(ctx, repository.DecideWithdrawalParams{
			ID: id, Status: decision, Comment: repository.TextPtr(nilIfEmpty(comment)),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: withdrawal_not_pending", platform.ErrConflict)
			}
			return err
		}
		out = claimed
		uid, err := tq.GetUserIDByPartnerID(ctx, claimed.PartnerID)
		if err != nil {
			return err
		}
		userID = uid
		if decision == "rejected" {
			wallet, err := tq.GetWalletByPartnerID(ctx, claimed.PartnerID)
			if err != nil {
				return err
			}
			updated, err := tq.ReleaseReserve(ctx, repository.ReleaseReserveParams{ID: wallet.ID, AvailableKopecks: claimed.AmountKopecks})
			if err != nil {
				return err
			}
			refundComment := "Заявка отклонена, средства возвращены"
			if _, err := tq.InsertLedgerEntry(ctx, repository.InsertLedgerEntryParams{
				WalletID: wallet.ID, Type: "withdrawal_refund", AmountKopecks: claimed.AmountKopecks,
				BalanceAfterKopecks: updated.AvailableKopecks,
				RefWithdrawalID:     repository.UUIDPtr(&claimed.ID),
				Comment:             repository.TextPtr(&refundComment),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return repository.WithdrawalRequest{}, err
	}
	title := "Заявка на вывод одобрена"
	body := "Заявка на вывод принята в обработку"
	if decision == "rejected" {
		title = "Заявка на вывод отклонена"
		body = "Средства возвращены на баланс"
	}
	_ = notifications.NotifyUser(ctx, s.q(ctx), userID, "withdrawal", title, body, nil)
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "withdrawal.decided", "withdrawal", id, map[string]any{"decision": decision, "comment": comment}, nil)
	}
	return out, nil
}

// Pay marks an approved withdrawal as paid and records the transfer.
func (s *Service) Pay(ctx context.Context, actorID *string, id, externalTxID, comment string) (repository.WithdrawalRequest, error) {
	var out repository.WithdrawalRequest
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		paid, err := tq.MarkWithdrawalPaid(ctx, repository.MarkWithdrawalPaidParams{
			ID: id, Comment: repository.TextPtr(nilIfEmpty(comment)),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: withdrawal_not_approved", platform.ErrConflict)
			}
			return err
		}
		out = paid
		if _, err := tq.InsertPayoutTransfer(ctx, repository.InsertPayoutTransferParams{
			WithdrawalRequestID: id, ExternalTxID: repository.TextPtr(nilIfEmpty(externalTxID)),
			AmountKopecks: paid.AmountKopecks, FeeKopecks: paid.FeeKopecks,
			CreatedBy: repository.UUIDPtr(actorID), Comment: repository.TextPtr(nilIfEmpty(comment)),
		}); err != nil {
			return err
		}
		wallet, err := tq.GetWalletByPartnerID(ctx, paid.PartnerID)
		if err != nil {
			return err
		}
		if _, err := tq.ConsumeReserve(ctx, repository.ConsumeReserveParams{ID: wallet.ID, ReservedKopecks: paid.AmountKopecks}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return repository.WithdrawalRequest{}, err
	}
	if uid, err := s.q(ctx).GetUserIDByPartnerID(ctx, out.PartnerID); err == nil {
		_ = notifications.NotifyUser(ctx, s.q(ctx), uid, "withdrawal", "Выплата произведена", "Заявка на вывод выплачена", nil)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "withdrawal.paid", "withdrawal", id, map[string]any{"external_tx_id": externalTxID}, nil)
	}
	return out, nil
}

// ListByPartner returns the partner's withdrawal requests.
func (s *Service) ListByPartner(ctx context.Context, partnerID string, limit int) ([]repository.WithdrawalRequest, error) {
	return s.q(ctx).ListWithdrawalsByPartner(ctx, repository.ListWithdrawalsByPartnerParams{PartnerID: partnerID, Limit: int32(limit)})
}

// ListAdmin returns withdrawal requests with partner info.
func (s *Service) ListAdmin(ctx context.Context, status, partnerID string, limit, offset int) ([]AdminWithdrawal, int64, error) {
	q := s.q(ctx)
	rows, err := q.ListWithdrawalsAdmin(ctx, repository.ListWithdrawalsAdminParams{
		Column1: status, Column2: partnerID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountWithdrawalsAdmin(ctx, repository.CountWithdrawalsAdminParams{Column1: status, Column2: partnerID})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminWithdrawal, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminWithdrawal{ListWithdrawalsAdminRow: r})
	}
	return out, total, nil
}

// AdminWithdrawal pairs a request row with partner info.
type AdminWithdrawal struct {
	repository.ListWithdrawalsAdminRow
}

func round8(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 {
		last := s[len(s)-1]
		if last == ' ' || last == '\t' || last == '\n' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return s
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
