package tenant

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// contextKey is an unexported type for context keys in this package.
type contextKey string

const tenantIDKey contextKey = "tenantID"

// Tenant represents a tenant in the multi-tenant system.
type Tenant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// TenantStore is an interface for tenant persistence.
type TenantStore interface {
	CreateTenant(ctx context.Context, tenant Tenant) error
	GetTenant(ctx context.Context, id string) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	DeleteTenant(ctx context.Context, id string) error
}

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext extracts the tenant ID from the context.
func TenantIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantIDKey).(string)
	return id
}

// IsolationMiddleware injects the tenant ID from the authenticated user's claims
// into the request context, enforcing tenant isolation.
func IsolationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenantID")
		if !exists || tenantID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant context required"})
			return
		}

		tid, ok := tenantID.(string)
		if !ok || tid == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid tenant context"})
			return
		}

		ctx := WithTenantID(c.Request.Context(), tid)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// FilterByTenant is a helper that appends a tenant filter condition to a query.
func FilterByTenant(baseQuery string, tenantID string) (string, []any) {
	if tenantID == "" {
		return baseQuery, nil
	}
	return fmt.Sprintf("%s AND tenant_id = $1", baseQuery), []any{tenantID}
}
