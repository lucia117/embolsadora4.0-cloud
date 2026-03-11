package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ResolveTenantFromPath is a middleware that resolves the tenant ID from the URL path parameter.
// It expects a :tenantId (subdomain slug) parameter and looks it up in the tenants table.
func ResolveTenantFromPath(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract :tenantId from path
		tenantSlug := c.Param("tenantId")
		if tenantSlug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tenantId required"})
			c.Abort()
			return
		}

		// Query tenants table by subdomain
		var tenantID uuid.UUID
		query := "SELECT id FROM tenants WHERE subdomain = $1"
		err := db.QueryRow(c.Request.Context(), query, tenantSlug).Scan(&tenantID)

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			c.Abort()
			return
		}

		// Set resolved UUID into context
		ctx := platform.WithTenantUUID(c.Request.Context(), tenantID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
