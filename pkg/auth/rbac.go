package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Role constants.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Permission represents an API action.
type Permission struct {
	Resource string
	Action   string
}

// rolePermissions defines which actions each role can perform.
var rolePermissions = map[string]map[string][]string{
	RoleAdmin: {
		"agents":    {"read", "write", "delete"},
		"tasks":     {"read", "write", "delete"},
		"workflows": {"read", "write", "delete"},
		"costs":     {"read"},
		"logs":      {"read"},
		"auth":      {"read", "write"},
		"tenants":   {"read", "write", "delete"},
	},
	RoleOperator: {
		"agents":    {"read", "write"},
		"tasks":     {"read", "write"},
		"workflows": {"read", "write"},
		"costs":     {"read"},
		"logs":      {"read"},
	},
	RoleViewer: {
		"agents":    {"read"},
		"tasks":     {"read"},
		"workflows": {"read"},
		"costs":     {"read"},
		"logs":      {"read"},
	},
}

// HasPermission checks whether a role has the specified permission.
func HasPermission(role, resource, action string) bool {
	resources, ok := rolePermissions[role]
	if !ok {
		return false
	}
	actions, ok := resources[resource]
	if !ok {
		return false
	}
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

// AuthMiddleware extracts and validates the JWT token from the Authorization header.
func AuthMiddleware(jwtMgr *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		claims, err := jwtMgr.ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("claims", claims)
		c.Set("userID", claims.Sub)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("tenantID", claims.TenantID)
		c.Next()
	}
}

// RequirePermission returns middleware that checks whether the authenticated user
// has the specified permission.
func RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no role found"})
			return
		}

		if !HasPermission(role.(string), resource, action) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}
		c.Next()
	}
}

// GetClaimsFromContext extracts claims from the gin context.
func GetClaimsFromContext(c *gin.Context) *Claims {
	val, exists := c.Get("claims")
	if !exists {
		return nil
	}
	claims, ok := val.(*Claims)
	if !ok {
		return nil
	}
	return claims
}
