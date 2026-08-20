// Package tracking implements click tokens, click recording and statistics.
package tracking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// clickTokenPayload is the signed payload embedded in a click token.
type clickTokenPayload struct {
	ClickID int64 `json:"click_id"`
	Exp     int64 `json:"exp"` // unix seconds
}

// ErrInvalidToken is returned when a click token fails signature/expiry checks.
var ErrInvalidToken = errors.New("invalid_click_token")

// SignClickToken creates payload.hmac token for a click.
func SignClickToken(secret string, ttl time.Duration, clickID int64) (string, error) {
	payload, err := json.Marshal(clickTokenPayload{ClickID: clickID, Exp: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sig, nil
}

// VerifyClickToken validates the token and returns the click ID.
func VerifyClickToken(secret, token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, ErrInvalidToken
	}
	var p clickTokenPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return 0, ErrInvalidToken
	}
	if p.ClickID <= 0 || time.Now().Unix() > p.Exp {
		return 0, ErrInvalidToken
	}
	return p.ClickID, nil
}

// MSK is the fixed Moscow timezone used for daily statistics boundaries.
var MSK = time.FixedZone("MSK", 3*3600)

// StartOfMSKDay returns the MSK midnight for the given time.
func StartOfMSKDay(t time.Time) time.Time {
	y, m, d := t.In(MSK).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, MSK)
}

// MSKDateKey formats a time as a MSK date string (YYYY-MM-DD).
func MSKDateKey(t time.Time) string {
	return t.In(MSK).Format("2006-01-02")
}

// MSKDayStart returns the MSK start of a date value.
func MSKDayStart(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, MSK)
}

var _ = fmt.Sprintf
