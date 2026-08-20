package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thecodearcher/limen"
	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

	"cashx/internal/outbox"
	"cashx/internal/platform"
	"cashx/internal/repository"
)

// Referral code alphabet without ambiguous characters.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Service implements the auth domain operations on top of Limen.
type Service struct {
	Pool  *pgxpool.Pool
	Limen *Limen
	Cfg   platform.Config
}

// q returns a Queries bound to the pool.
func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Register creates a partner account in pending state via Limen's programmatic
// signup, then binds our domain rows (partner profile, wallet, referral).
// No session is created: the partner must be approved first.
func (s *Service) Register(ctx context.Context, name, emailRaw, password, referralCode string) (*repository.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	if !emailRe.MatchString(email) {
		return nil, fmt.Errorf("%w: invalid_email", platform.ErrValidation)
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: invalid_password", platform.ErrValidation)
	}

	// Referral handling: a partner can invite others with their referral code.
	var referredBy *string
	if strings.TrimSpace(referralCode) != "" {
		code := normalizeCode(referralCode)
		inviter, err := s.q(ctx).GetPartnerProfileByReferralCode(ctx, code)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: invalid_referral_code", platform.ErrValidation)
			}
			return nil, err
		}
		referredBy = &inviter.ID
	}

	// Limen creates the user (Argon2id hash in users.password); the role and
	// partner flag are our domain fields.
	pwd := password
	res, err := s.Limen.Password.SignUpWithCredentialAndPassword(ctx,
		&limen.User{Email: email, Password: &pwd},
		map[string]any{"name": name, "role": "partner", "is_active": true},
	)
	if err != nil {
		if errors.Is(err, credentialpassword.ErrEmailAlreadyExists) {
			return nil, fmt.Errorf("%w: email_taken", platform.ErrConflict)
		}
		return nil, err
	}

	refCode, err := GenerateReferralCode(ctx, s.q(ctx))
	if err != nil {
		return nil, err
	}

	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		profile, err := tq.CreatePartnerProfile(ctx, repository.CreatePartnerProfileParams{
			UserID:       fmt.Sprint(res.User.ID),
			ReferralCode: refCode,
			ReferredBy:   repository.UUIDPtr(referredBy),
		})
		if err != nil {
			return err
		}
		if _, err := tq.CreateWallet(ctx, profile.ID); err != nil {
			return err
		}
		if referredBy != nil {
			// Referral rate snapshot from platform settings (default 250 bps).
			rate := 250
			if v, err := platform.GetIntSetting(ctx, tq, "referral_rate_default_bps"); err == nil && v > 0 {
				rate = v
			}
			if _, err := tq.CreatePartnerReferral(ctx, repository.CreatePartnerReferralParams{
				ReferrerPartnerID: *referredBy,
				InvitedPartnerID:  profile.ID,
				ReferralRateBps:   int32(rate),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	u, err := s.q(ctx).GetUserByID(ctx, fmt.Sprint(res.User.ID))
	if err != nil {
		return nil, err
	}
	return &repository.User{
		ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, IsActive: u.IsActive,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}, nil
}

// PasswordResetRequest issues a reset token via Limen; always succeeds from
// the caller's perspective (does not reveal whether the email exists). Returns
// the dev-only token (empty when not in dev).
func (s *Service) PasswordResetRequest(ctx context.Context, emailRaw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	v, err := s.Limen.Password.RequestPasswordReset(ctx, email)
	if err != nil {
		return "", nil // do not leak existence
	}
	token := v.Value
	if s.Cfg.Env == "development" {
		return token, nil
	}
	_ = outbox.Enqueue(ctx, s.q(ctx), "email", map[string]string{
		"to": email, "subject": "CashX: сброс пароля",
		"body": "Перейдите по ссылке, чтобы сбросить пароль: " + s.resetURL(token),
	})
	return "", nil
}

// PasswordResetConfirm resets the password through Limen. A token that is
// invalid, expired or already consumed yields invalid_token.
func (s *Service) PasswordResetConfirm(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("%w: invalid_password", platform.ErrValidation)
	}
	if err := s.Limen.Password.ResetPassword(ctx, token, newPassword); err != nil {
		return fmt.Errorf("%w: invalid_token", platform.ErrValidation)
	}
	return nil
}

// GenerateReferralCode returns a fresh, unique 8-character referral code from
// the unambiguous alphabet. Shared by partner registration and staff profile
// seeding so every account gets the same short, human-friendly format.
func GenerateReferralCode(ctx context.Context, q *repository.Queries) (string, error) {
	for i := 0; i < 8; i++ {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, b := range buf {
			sb.WriteByte(codeAlphabet[int(b)%len(codeAlphabet)])
		}
		code := sb.String()
		if _, err := q.GetPartnerProfileByReferralCode(ctx, code); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return code, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("could not generate unique referral code")
}

func normalizeCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func (s *Service) resetURL(token string) string {
	return s.Cfg.WebOrigin + "/reset?token=" + token
}
