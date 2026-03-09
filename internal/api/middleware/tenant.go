package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantID is the context key for tenant ID
const TenantID = "tenant_id"

// ExtractTenantID extracts X-Tenant-ID header and validates it
func ExtractTenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "MISSING_HEADER",
				"message": "X-Tenant-ID header is required",
				"status":  http.StatusBadRequest,
			})
			c.Abort()
			return
		}

		// Store in context for handlers to access
		c.Set(TenantID, tenantID)
		c.Next()
	}
}
