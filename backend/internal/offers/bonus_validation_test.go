package offers

import (
	"context"
	"errors"
	"testing"

	"cashx/internal/platform"
)

func TestNegativeBonusRejected(t *testing.T) {
	svc := &Service{}
	negative := -100
	validBonus := 1500

	ctx := context.Background()

	// Link with negative bonus
	_, err := svc.CreateLinkSource(ctx, "p1", "o1", "test-link", nil, &negative, nil, nil, nil, nil, false)
	if err == nil || !errors.Is(err, platform.ErrValidation) {
		t.Fatalf("expected ErrValidation for link with negative bonus, got %v", err)
	}

	// Promo with negative bonus
	_, err = svc.CreatePromoSource(ctx, "p1", "o1", "test-promo", nil, &negative, nil, nil)
	if err == nil || !errors.Is(err, platform.ErrValidation) {
		t.Fatalf("expected ErrValidation for promo with negative bonus, got %v", err)
	}

	// Empty name rejected
	_, err = svc.CreateLinkSource(ctx, "p1", "o1", "   ", nil, &validBonus, nil, nil, nil, nil, false)
	if err == nil || !errors.Is(err, platform.ErrValidation) {
		t.Fatalf("expected ErrValidation for empty name, got %v", err)
	}
}
