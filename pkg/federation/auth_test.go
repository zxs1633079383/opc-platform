package federation

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- TestGenerateAPIKey ---

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()

	// 32 bytes -> 64 hex chars.
	if len(key) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars: %s", len(key), key)
	}

	// Verify it's valid hex.
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character in key: %c", c)
		}
	}

	// Two generated keys should be different.
	key2 := GenerateAPIKey()
	if key == key2 {
		t.Error("two generated keys should not be identical")
	}
}

// --- TestSignRequest_Verify ---

func TestSignRequest_Verify(t *testing.T) {
	apiKey := GenerateAPIKey()
	body := []byte(`{"action": "deploy"}`)
	ts := time.Now().Unix()

	sig := SignRequest(body, apiKey, ts)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	// Build an HTTP request with proper headers.
	req, err := http.NewRequest("POST", "http://example.com/api/v1/tasks", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-OPC-Signature", sig)
	req.Header.Set("X-OPC-Timestamp", fmt.Sprintf("%d", ts))

	if err := VerifyRequest(req, apiKey); err != nil {
		t.Fatalf("verify should succeed: %v", err)
	}

	// Verify body is still readable after verification.
	restoredBody, _ := io.ReadAll(req.Body)
	if string(restoredBody) != string(body) {
		t.Error("body should be restored after verification")
	}
}

// --- TestVerifyRequest_InvalidSignature ---

func TestVerifyRequest_InvalidSignature(t *testing.T) {
	apiKey := GenerateAPIKey()
	body := []byte(`{"data": "test"}`)
	ts := time.Now().Unix()

	req, _ := http.NewRequest("POST", "http://example.com/api", strings.NewReader(string(body)))
	req.Header.Set("X-OPC-Signature", "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678")
	req.Header.Set("X-OPC-Timestamp", fmt.Sprintf("%d", ts))

	err := VerifyRequest(req, apiKey)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
	if !strings.Contains(err.Error(), "invalid signature") {
		t.Errorf("expected 'invalid signature' error, got: %v", err)
	}
}

// --- TestVerifyRequest_MissingHeaders ---

func TestVerifyRequest_MissingHeaders(t *testing.T) {
	apiKey := GenerateAPIKey()

	tests := []struct {
		name      string
		signature string
		timestamp string
	}{
		{"missing both", "", ""},
		{"missing signature", "", fmt.Sprintf("%d", time.Now().Unix())},
		{"missing timestamp", "somesig", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "http://example.com/api", strings.NewReader("{}"))
			if tc.signature != "" {
				req.Header.Set("X-OPC-Signature", tc.signature)
			}
			if tc.timestamp != "" {
				req.Header.Set("X-OPC-Timestamp", tc.timestamp)
			}

			err := VerifyRequest(req, apiKey)
			if err == nil {
				t.Fatal("expected error for missing headers")
			}
			if !strings.Contains(err.Error(), "missing auth headers") {
				t.Errorf("expected 'missing auth headers' error, got: %v", err)
			}
		})
	}
}

// --- TestVerifyRequest_ExpiredTimestamp ---

func TestVerifyRequest_ExpiredTimestamp(t *testing.T) {
	apiKey := GenerateAPIKey()
	body := []byte(`{"data": "old"}`)
	// Timestamp 6 minutes ago (>5 min threshold).
	ts := time.Now().Unix() - 360

	sig := SignRequest(body, apiKey, ts)

	req, _ := http.NewRequest("POST", "http://example.com/api", strings.NewReader(string(body)))
	req.Header.Set("X-OPC-Signature", sig)
	req.Header.Set("X-OPC-Timestamp", fmt.Sprintf("%d", ts))

	err := VerifyRequest(req, apiKey)
	if err == nil {
		t.Fatal("expected error for expired timestamp")
	}
	if !strings.Contains(err.Error(), "request too old") {
		t.Errorf("expected 'request too old' error, got: %v", err)
	}
}

// --- TestSignRequest_Deterministic ---

func TestSignRequest_Deterministic(t *testing.T) {
	apiKey := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	body := []byte(`{"hello": "world"}`)
	ts := int64(1700000000)

	sig1 := SignRequest(body, apiKey, ts)
	sig2 := SignRequest(body, apiKey, ts)

	if sig1 != sig2 {
		t.Error("same inputs should produce same signature")
	}

	// Different body should produce different signature.
	sig3 := SignRequest([]byte(`{"different": "body"}`), apiKey, ts)
	if sig1 == sig3 {
		t.Error("different body should produce different signature")
	}

	// Different timestamp should produce different signature.
	sig4 := SignRequest(body, apiKey, ts+1)
	if sig1 == sig4 {
		t.Error("different timestamp should produce different signature")
	}
}
