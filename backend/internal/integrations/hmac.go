// Package integrations implements the signed project event API (HMAC),
// including attribution, commissions and referral rewards.
package integrations

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const maxBodySize = 1 << 20 // 1 MiB

// MaxSkew is the allowed timestamp difference for signed requests.
const MaxSkew = 300 * time.Second

// verify computes and compares the request signature. body is the raw request
// body; the canonical string is "<unix-ts>.<body>".
func verify(secret, tsRaw, sigRaw, body string) error {
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return errSignature
	}
	if d := time.Since(time.Unix(ts, 0)); d < -MaxSkew || d > MaxSkew {
		return errSignature
	}
	canonical := tsRaw + "." + body
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sigRaw)) {
		return errSignature
	}
	return nil
}

var errSignature = fmt.Errorf("invalid_signature")

// readBody reads and limits the request body.
func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
}
