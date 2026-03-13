package middleware

import (
	"github.com/gin-gonic/gin"
)

// UserRole is the context key for user role
const UserRole = "user_role"

// RequireRole creates a middleware that checks if user has required role
// TODO: Implement proper JWT extraction and role validation
// For now, this is disabled to allow easier testing
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Extract role from JWT token and validate
		// Temporarily disabled for testing - will be implemented in production
		// In production:
		// - Extract JWT from Authorization header
		// - Parse and validate token claims
		// - Check if user has required role
		// - Deny if insufficient permissions

		c.Next()
	}
}

// SetUserRoleFromJWT sets the user role from JWT token (placeholder)
// In production, this should extract from verified JWT claims
func SetUserRoleFromJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Extract role from JWT token
		// For now, this is a placeholder
		// In real implementation:
		// claims := c.Get("claims") // from JWT middleware
		// role := claims.Role
		// c.Set(UserRole, role)

		c.Next()
	}
}
