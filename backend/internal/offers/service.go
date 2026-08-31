// Package offers implements the offer domain: offers, terms, partner accesses,
// tracking links, integration keys.
package offers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/audit"
	"cashx/internal/platform"
	"cashx/internal/repository"
)

const linkCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Service implements offer operations.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.Recorder
	// IntegrationEncryptionKey encrypts integration key secrets (sha256'd).
	IntegrationEncryptionKey string
	// WebOrigin is used to build tracking URLs.
	WebOrigin string
}

func (s *Service) q(ctx context.Context) *repository.Queries { return repository.New(s.Pool) }

// Card is the API shape of an offer.
type Card struct {
	ID             string  `json:"id"`
	ProjectID      string  `json:"project_id"`
	ProjectName    string  `json:"project_name"`
	Name           string  `json:"name"`
	Category       *string `json:"category"`
	Description    *string `json:"description"`
	DestinationURL *string `json:"destination_url"`
	Status         string  `json:"status"`
	CurrentRateBps int     `json:"current_rate_bps"`
	Version        int     `json:"version"`
	CreatedAt      string  `json:"created_at"`
}

// Create inserts an offer with terms v1. One active offer per project is
// enforced by the partial unique index.
func (s *Service) Create(ctx context.Context, actorID *string, projectID, name string, category, description, destinationURL *string, status string, rateBps int) (Card, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Card{}, fmt.Errorf("%w: invalid_name", platform.ErrValidation)
	}
	if rateBps < 0 || rateBps > 10000 {
		return Card{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
	}
	if status == "" {
		status = "pending"
	}
	var card Card
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		o, err := tq.CreateOffer(ctx, repository.CreateOfferParams{
			ProjectID: projectID, Name: name,
			Category:       repository.TextPtr(category),
			Description:    repository.TextPtr(description),
			DestinationUrl: repository.TextPtr(destinationURL),
			Status:         status,
		})
		if err != nil {
			return err
		}
		if _, err := tq.CreateOfferTerms(ctx, repository.CreateOfferTermsParams{
			OfferID: o.ID, Version: 1, RateBps: int32(rateBps), EffectiveFrom: repository.TimePtr(&timeNow),
		}); err != nil {
			return err
		}
		card, err = s.cardFor(ctx, tq, o.ID)
		return err
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Card{}, fmt.Errorf("%w: offer_already_active", platform.ErrConflict)
		}
		return Card{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "offer.created", "offer", card.ID, map[string]any{"name": name, "rate_bps": rateBps}, nil)
	}
	return card, nil
}

var timeNow = time.Now()

// Update patches the offer card. Setting status to active may hit the one-active
// offer constraint.
func (s *Service) Update(ctx context.Context, actorID *string, id string, name, category, description, destinationURL *string, status *string) (Card, error) {
	var card Card
	err := repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		o, err := tq.UpdateOffer(ctx, repository.UpdateOfferParams{
			ID:             id,
			Name:           repository.TextPtr(name),
			Category:       repository.TextPtr(category),
			Description:    repository.TextPtr(description),
			DestinationUrl: repository.TextPtr(destinationURL),
			Status:         repository.TextPtr(status),
		})
		if err != nil {
			return err
		}
		card, err = s.cardFor(ctx, tq, o.ID)
		return err
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, fmt.Errorf("%w: offer_not_found", platform.ErrNotFound)
		}
		if isUniqueViolation(err) {
			return Card{}, fmt.Errorf("%w: offer_already_active", platform.ErrConflict)
		}
		return Card{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "offer.updated", "offer", card.ID, map[string]any{"status": status, "name": name}, nil)
	}
	return card, nil
}

// AddTerms appends a new terms version (rate change for future joins).
func (s *Service) AddTerms(ctx context.Context, actorID *string, id string, rateBps int) (version int, effectiveFrom time.Time, err error) {
	if rateBps < 0 || rateBps > 10000 {
		return 0, time.Time{}, fmt.Errorf("%w: invalid_rate", platform.ErrValidation)
	}
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		n, err := tq.CountOfferTerms(ctx, id)
		if err != nil {
			return err
		}
		eff := time.Now()
		row, err := tq.CreateOfferTerms(ctx, repository.CreateOfferTermsParams{
			OfferID: id, Version: int32(n + 1), RateBps: int32(rateBps), EffectiveFrom: repository.TimePtr(&eff),
		})
		if err != nil {
			return err
		}
		version = int(row.Version)
		effectiveFrom = eff
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, time.Time{}, fmt.Errorf("%w: offer_not_found", platform.ErrNotFound)
		}
		return 0, time.Time{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "offer.terms_changed", "offer", id, map[string]any{"version": version, "rate_bps": rateBps}, nil)
	}
	return version, effectiveFrom, nil
}

// List returns offers, optionally filtered by project.
func (s *Service) List(ctx context.Context, projectID string, limit, offset int) ([]Card, int64, error) {
	q := s.q(ctx)
	rows, err := q.ListOffers(ctx, repository.ListOffersParams{
		Column1: projectID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountOffers(ctx, projectID)
	if err != nil {
		return nil, 0, err
	}
	cards := make([]Card, 0, len(rows))
	for _, r := range rows {
		cards = append(cards, Card{
			ID: r.ID, ProjectID: r.ProjectID, ProjectName: r.ProjectName, Name: r.Name,
			Category: repository.TextToPtr(r.Category), Description: repository.TextToPtr(r.Description),
			DestinationURL: repository.TextToPtr(r.DestinationUrl), Status: r.Status,
			CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return cards, total, nil
}

// Get returns the full card (with current terms) for an offer.
func (s *Service) Get(ctx context.Context, id string) (Card, error) {
	return s.cardFor(ctx, s.q(ctx), id)
}

func (s *Service) cardFor(ctx context.Context, q *repository.Queries, id string) (Card, error) {
	o, err := q.GetOfferWithProject(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || (err != nil && err.Error() == "no rows in result set") {
			return Card{}, fmt.Errorf("%w: offer_not_found", platform.ErrNotFound)
		}
		return Card{}, err
	}
	terms, err := q.GetCurrentTerms(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Card{}, err
	}
	card := Card{
		ID: o.ID, ProjectID: o.ProjectID, ProjectName: o.ProjectName, Name: o.Name,
		Category: repository.TextToPtr(o.Category), Description: repository.TextToPtr(o.Description),
		DestinationURL: repository.TextToPtr(o.DestinationUrl), Status: o.Status,
		CreatedAt: o.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if err == nil {
		card.CurrentRateBps = int(terms.RateBps)
		card.Version = int(terms.Version)
	}
	return card, nil
}

// Join connects a partner to an active offer, creating the partner's access
// and a tracking link. The access rate is the partner's single RevShare
// percent (same for every offer), not the offer's own terms rate.
func (s *Service) Join(ctx context.Context, partnerID, offerID string) (rateBps int, trackingURL string, err error) {
	q := s.q(ctx)
	offer, err := q.GetOfferWithProject(ctx, offerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", fmt.Errorf("%w: offer_not_found", platform.ErrNotFound)
		}
		return 0, "", err
	}
	if offer.Status != "active" {
		return 0, "", fmt.Errorf("%w: offer_not_joinable", platform.ErrConflict)
	}
	settings, err := q.GetProjectSettings(ctx, offer.ProjectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", err
	}
	if err == nil && !settings.AllowNewPartners {
		return 0, "", fmt.Errorf("%w: offer_not_joinable", platform.ErrConflict)
	}
	if _, err := q.GetPartnerAccess(ctx, repository.GetPartnerAccessParams{PartnerID: partnerID, OfferID: offerID}); err == nil {
		return 0, "", fmt.Errorf("%w: already_joined", platform.ErrConflict)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", err
	}
	// Partner-level RevShare applies to every offer the partner joins.
	profile, err := q.GetPartnerProfileByID(ctx, partnerID)
	if err != nil {
		return 0, "", err
	}
	code, err := genLinkCode(ctx, q)
	if err != nil {
		return 0, "", err
	}
	var link repository.TrackingLink
	err = repository.WithTx(ctx, s.Pool, func(tq *repository.Queries) error {
		access, err := tq.CreatePartnerAccess(ctx, repository.CreatePartnerAccessParams{
			PartnerID: partnerID, OfferID: offerID, RateBps: profile.RevsharePercentBps,
		})
		if err != nil {
			return err
		}
		link, err = tq.CreateTrackingLink(ctx, repository.CreateTrackingLinkParams{
			PartnerOfferAccessID: access.ID,
			Code:                 code,
			Name:                 "Основной источник",
			IsDefault:            true,
		})
		return err
	})
	if err != nil {
		if isUniqueViolation(err) {
			return 0, "", fmt.Errorf("%w: already_joined", platform.ErrConflict)
		}
		return 0, "", err
	}
	base := s.WebOrigin
	if base == "" {
		base = "http://localhost:3000"
	}
	return int(profile.RevsharePercentBps), base + "/c/" + link.Code, nil
}

func genLinkCode(ctx context.Context, q *repository.Queries) (string, error) {
	for i := 0; i < 8; i++ {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, b := range buf {
			sb.WriteByte(linkCodeAlphabet[int(b)%len(linkCodeAlphabet)])
		}
		code := sb.String()
		if _, err := q.GetTrackingLinkByCode(ctx, code); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return code, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("could not generate unique link code")
}

// ---------------------------------------------------------------------------
// Integration keys.

// SecretPair is a created/rotated key with its one-time secret.
type SecretPair struct {
	KeyID  string `json:"key_id"`
	Secret string `json:"secret"`
}

// KeyCard is a key row without the secret.
type KeyCard struct {
	KeyID      string  `json:"key_id"`
	IsActive   bool    `json:"is_active"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
	SecretHint string  `json:"secret_hint"`
}

func (s *Service) encryptionKey() []byte {
	sum := sha256.Sum256([]byte(s.IntegrationEncryptionKey))
	return sum[:]
}

// EncryptSecret AES-256-GCM encrypts a secret for storage.
func (s *Service) EncryptSecret(secret string) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey())
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(secret), nil), nil
}

// DecryptSecret reverses EncryptSecret.
func (s *Service) DecryptSecret(ct []byte) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ct) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, body := ct[:gcm.NonceSize()], ct[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// CreateKey creates an integration key for a project and returns the secret
// (shown only once).
func (s *Service) CreateKey(ctx context.Context, actorID *string, projectID string) (SecretPair, error) {
	secret, err := randomSecret()
	if err != nil {
		return SecretPair{}, err
	}
	keyID, err := randomSecret()
	if err != nil {
		return SecretPair{}, err
	}
	ct, err := s.EncryptSecret(secret)
	if err != nil {
		return SecretPair{}, err
	}
	row, err := s.q(ctx).CreateIntegrationKey(ctx, repository.CreateIntegrationKeyParams{
		ProjectID: projectID, KeyID: keyID,
		SecretCiphertext: ct, SecretHint: hint(secret),
	})
	if err != nil {
		return SecretPair{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "integration_key.created", "integration_key", row.KeyID, map[string]any{"project_id": projectID}, nil)
	}
	return SecretPair{KeyID: row.KeyID, Secret: secret}, nil
}

// RotateKey deactivates the old key and creates a fresh one.
func (s *Service) RotateKey(ctx context.Context, actorID *string, keyID string) (SecretPair, error) {
	old, err := s.q(ctx).GetIntegrationKeyByKeyID(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SecretPair{}, fmt.Errorf("%w: key_not_found", platform.ErrNotFound)
		}
		return SecretPair{}, err
	}
	pair, err := s.CreateKey(ctx, actorID, old.ProjectID)
	if err != nil {
		return SecretPair{}, err
	}
	if err := s.q(ctx).DeactivateIntegrationKey(ctx, keyID); err != nil {
		return SecretPair{}, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "integration_key.rotated", "integration_key", keyID, nil, nil)
	}
	return pair, nil
}

// DeactivateKey disables a key.
func (s *Service) DeactivateKey(ctx context.Context, actorID *string, keyID string) error {
	row, err := s.q(ctx).GetIntegrationKeyByKeyID(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: key_not_found", platform.ErrNotFound)
		}
		return err
	}
	if err := s.q(ctx).DeactivateIntegrationKey(ctx, keyID); err != nil {
		return err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, actorID, "integration_key.deactivated", "integration_key", row.KeyID, nil, nil)
	}
	return nil
}

// ListKeys returns keys without secrets.
func (s *Service) ListKeys(ctx context.Context, projectID string) ([]KeyCard, error) {
	rows, err := s.q(ctx).ListIntegrationKeysByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]KeyCard, 0, len(rows))
	for _, r := range rows {
		last := repository.TimeToPtr(r.LastUsedAt)
		var lastStr *string
		if last != nil {
			s := last.UTC().Format(time.RFC3339)
			lastStr = &s
		}
		out = append(out, KeyCard{
			KeyID: r.KeyID, IsActive: r.IsActive,
			CreatedAt:  r.CreatedAt.Time.UTC().Format(time.RFC3339),
			LastUsedAt: lastStr, SecretHint: r.SecretHint,
		})
	}
	return out, nil
}

func randomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hint(secret string) string {
	if len(secret) <= 4 {
		return secret
	}
	return secret[len(secret)-4:]
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
