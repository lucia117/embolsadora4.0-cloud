package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// ResolveTenantAndCheckMembership resolves the tenant from the :tenantId subdomain
// slug, then authorizes the caller: a direct active membership, or (fallback) a
// platform operator (global role / admin of the platform tenant) acting
// cross-tenant per ADR-015. On success it injects tenant UUID, tenant ID string,
// and effective role into context. Unknown subdomain -> 404; authorized but not
// a member -> role from resolvePlatformOperator; neither -> 403.
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

		// 1. Resolver subdomain -> (tenant UUID, is_platform_tenant).
		var tenantIDStr string
		var isPlatformTenant bool
		err := db.QueryRow(c.Request.Context(),
			`SELECT id::text, is_platform_tenant FROM tenants WHERE subdomain = $1 LIMIT 1`,
			tenantSlug,
		).Scan(&tenantIDStr, &isPlatformTenant)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				Log.Warn("tenant not found",
					zap.String("user_id", user.ID),
					zap.String("tenant_slug", tenantSlug),
					zap.String("endpoint", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "error": "tenant not found"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
			return
		}

		tenantUUID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
			return
		}

		// 2. Membresía directa activa del usuario en ese tenant.
		var roleID string
		err = db.QueryRow(c.Request.Context(),
			`SELECT role_id FROM user_tenant_roles
			 WHERE tenant_id = $1 AND user_id = $2 AND status = 'active'
			 LIMIT 1`,
			tenantUUID, user.ID,
		).Scan(&roleID)

		switch {
		case err == nil:
			roleID = security.EffectiveRole(roleID, isPlatformTenant)
		case errors.Is(err, pgx.ErrNoRows):
			// 3. Fallback: operador de plataforma sin membresía directa
			//    (rol global, o admin del tenant plataforma). Ver
			//    docs/adr/ADR-015-plataforma-cross-tenant.md.
			roleID, err = resolvePlatformOperator(c.Request.Context(), db, user.ID, tenantIDStr)
			if err != nil {
				if errors.Is(err, errTargetTenantNotFound) {
					// No debería pasar (ya validamos existencia en el paso 1),
					// pero se mapea consistente por robustez.
					c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "error": "tenant not found"})
					return
				}
				Log.Warn("tenant access denied",
					zap.String("user_id", user.ID),
					zap.String("tenant_slug", tenantSlug),
					zap.String("endpoint", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "tenant access denied"})
				return
			}
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
			return
		}

		permissions, isGlobal, err := loadRolePermissions(c.Request.Context(), db, roleID)
		if err != nil {
			Log.Error("failed to load role permissions",
				zap.String("role_id", roleID),
				zap.String("user_id", user.ID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal error"})
			return
		}

		ctx := platform.WithTenantID(c.Request.Context(), tenantIDStr)
		ctx = platform.WithTenantUUID(ctx, tenantUUID)
		ctx = security.WithRoleContext(ctx, security.RoleContext{
			Name:        roleID,
			Permissions: permissions,
			IsGlobal:    isGlobal,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
