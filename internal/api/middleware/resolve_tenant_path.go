package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// ResolveTenantAndCheckMembership resolves the tenant from the :tenantId subdomain slug
// and verifies in one query that the authenticated user has an active membership in it.
// On success it injects tenant UUID, tenant ID string, and role into context.
func ResolveTenantAndCheckMembership(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantSlug := c.Param("tenantId")
		if tenantSlug == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenantId required"})
			return
		}

		user, ok := platform.DomainUser(c.Request.Context()).(*domain.User)
		if !ok || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
			return
		}

		var tenantIDStr string
		var roleID string
		err := db.QueryRow(c.Request.Context(), `
			SELECT t.id::text, utr.role_id
			FROM tenants t
			JOIN user_tenant_roles utr ON utr.tenant_id = t.id
			WHERE t.subdomain   = $1
			  AND utr.user_id   = $2
			  AND utr.status    = 'active'
			LIMIT 1`,
			tenantSlug, user.ID,
		).Scan(&tenantIDStr, &roleID)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Could be unknown tenant or user not a member — same 403 to avoid enumeration.
				Log.Warn("tenant access denied",
					zap.String("user_id", user.ID),
					zap.String("tenant_slug", tenantSlug),
					zap.String("endpoint", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "tenant access denied"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
			return
		}

		ctx := platform.WithTenantID(c.Request.Context(), tenantIDStr)
		ctx = security.WithRole(ctx, roleID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
