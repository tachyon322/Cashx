package tracking

import (
	"testing"
	"time"
)

func TestSignAndVerifyClickToken(t *testing.T) {
	secret := "test-secret-key-12345"
	clickID := int64(987654)
	ttl := 1 * time.Hour

	token, err := SignClickToken(secret, ttl, clickID)
	if err != nil {
		t.Fatalf("SignClickToken failed: %v", err)
	}

	gotID, err := VerifyClickToken(secret, token)
	if err != nil {
		t.Fatalf("VerifyClickToken failed: %v", err)
	}
	if gotID != clickID {
		t.Fatalf("expected clickID %d, got %d", clickID, gotID)
	}
}

func TestVerifyExpiredClickToken(t *testing.T) {
	secret := "test-secret-key-12345"
	clickID := int64(12345)
	ttl := -1 * time.Second // already expired

	token, err := SignClickToken(secret, ttl, clickID)
	if err != nil {
		t.Fatalf("SignClickToken failed: %v", err)
	}

	_, err = VerifyClickToken(secret, token)
	if err == nil {
		t.Fatalf("expected error for expired token, got nil")
	}
}

func TestVerifyTamperedClickToken(t *testing.T) {
	secret := "test-secret-key-12345"
	wrongSecret := "another-secret-key-67890"
	clickID := int64(12345)
	ttl := 1 * time.Hour

	token, err := SignClickToken(secret, ttl, clickID)
	if err != nil {
		t.Fatalf("SignClickToken failed: %v", err)
	}

	_, err = VerifyClickToken(wrongSecret, token)
	if err == nil {
		t.Fatalf("expected error for token verified with wrong secret, got nil")
	}

	_, err = VerifyClickToken(secret, token+"tampered")
	if err == nil {
		t.Fatalf("expected error for tampered token, got nil")
	}

	_, err = VerifyClickToken(secret, "invalid-token-without-dot")
	if err == nil {
		t.Fatalf("expected error for malformed token, got nil")
	}
}
