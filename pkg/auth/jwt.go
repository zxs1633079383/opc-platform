package auth

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

var (
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenInvalid  = errors.New("invalid token")
	ErrTokenMalformed = errors.New("malformed token")
)

// Claims represents the JWT payload.
type Claims struct {
	Sub      string   `json:"sub"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	TenantID string   `json:"tenantId,omitempty"`
	Exp      int64    `json:"exp"`
	Iat      int64    `json:"iat"`
}

// JWTManager handles JWT token generation and validation.
type JWTManager struct {
	secret     []byte
	expiration time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

// GenerateToken creates a new JWT token for the given user.
func (m *JWTManager) GenerateToken(userID, username, role, tenantID string) (string, error) {
	now := time.Now()
	claims := Claims{
		Sub:      userID,
		Username: username,
		Role:     role,
		TenantID: tenantID,
		Iat:      now.Unix(),
		Exp:      now.Add(m.expiration).Unix(),
	}

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	headerEncoded := base64URLEncode(headerJSON)
	claimsEncoded := base64URLEncode(claimsJSON)
	signingInput := headerEncoded + "." + claimsEncoded

	signature := m.sign([]byte(signingInput))
	signatureEncoded := base64URLEncode(signature)

	return signingInput + "." + signatureEncoded, nil
}

// ValidateToken parses and validates a JWT token, returning the claims.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, ErrTokenMalformed
	}

	expectedSig := m.sign([]byte(signingInput))
	if !hmac.Equal(signature, expectedSig) {
		return nil, ErrTokenInvalid
	}

	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrTokenMalformed
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func (m *JWTManager) sign(data []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write(data)
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
