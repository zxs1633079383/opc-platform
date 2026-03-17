package federation

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GenerateAPIKey creates a cryptographically random 32-byte hex API key.
func GenerateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SignRequest computes an HMAC-SHA256 signature over the timestamp and body.
func SignRequest(body []byte, apiKey string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(fmt.Sprintf("%d", timestamp)))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyRequest validates the HMAC-SHA256 signature in the request headers.
// It reads X-OPC-Signature and X-OPC-Timestamp headers and verifies them
// against the provided apiKey. The request body is read and restored.
func VerifyRequest(r *http.Request, apiKey string) error {
	sig := r.Header.Get("X-OPC-Signature")
	tsStr := r.Header.Get("X-OPC-Timestamp")
	if sig == "" || tsStr == "" {
		return fmt.Errorf("missing auth headers")
	}

	// Read body, verify, then restore for downstream handlers.
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	var ts int64
	fmt.Sscanf(tsStr, "%d", &ts)

	// Reject requests older than 5 minutes.
	if time.Now().Unix()-ts > 300 {
		return fmt.Errorf("request too old")
	}

	expected := SignRequest(body, apiKey, ts)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
