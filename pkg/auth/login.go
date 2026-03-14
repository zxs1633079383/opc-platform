package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// User represents a user account.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
	Role     string `json:"role"`
	TenantID string `json:"tenantId,omitempty"`
}

// UserStore is an interface for user persistence.
type UserStore interface {
	GetUserByUsername(username string) (*User, error)
	CreateUser(user User) error
}

// LoginRequest is the login API request body.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the login API response body.
type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// LoginHandler returns a gin handler for the login endpoint.
func LoginHandler(jwtMgr *JWTManager, userStore UserStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		user, err := userStore.GetUserByUsername(req.Username)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if HashPassword(req.Password) != user.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := jwtMgr.GenerateToken(user.ID, user.Username, user.Role, user.TenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, LoginResponse{
			Token:    token,
			Username: user.Username,
			Role:     user.Role,
		})
	}
}

// HashPassword returns the SHA-256 hash of a password.
func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}
