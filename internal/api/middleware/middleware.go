package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Log is the package-level Zap logger. Set via SetLogger during application startup.
var Log *zap.Logger = zap.NewNop()

// JWTAuth validates the Supabase JWT, auto-provisions the user on first login,
// and injects supabase sub + domain.User into the request context.
// Pass an InvitationActivator to activate pending invitations on first login (nil = disabled).
func JWTAuth(verifier security.Verifier, authUC *usecases.AuthUsecase, activator usecases.InvitationActivator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "No autorizado"})
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := verifier.Verify(tokenString)
		if err != nil {
			if errors.Is(err, security.ErrJWKSUnavailable) {
				Log.Error("JWKS endpoint unavailable", zap.String("endpoint", c.Request.URL.Path), zap.Error(err))
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
				return
			}
			Log.Warn("invalid JWT token", zap.String("endpoint", c.Request.URL.Path), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "No autorizado"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "No autorizado"})
			return
		}

		sub, _ := claims["sub"].(string)
		email, _ := claims["email"].(string)

		if sub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "No autorizado"})
			return
		}

		// Inject Supabase sub into context
		ctx := platform.WithSupabaseSub(c.Request.Context(), sub)

		// Auto-provision user (idempotent upsert)
		user, err := authUC.ProvisionUser(ctx, sub, email)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "provisioning failed"})
			return
		}

		// Status check: revoked or disabled accounts are rejected
		if user.Status == domain.UserStatusRevoked || user.Status == domain.UserStatusDisabled {
			Log.Warn("account suspended",
				zap.String("supabase_user_id", sub),
				zap.String("status", string(user.Status)),
				zap.String("endpoint", c.Request.URL.Path),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account suspended"})
			return
		}

		// Inject user ID (UUID) for backward compatibility
		if id, err := uuid.Parse(user.ID); err == nil {
			ctx = platform.WithUserID(ctx, id)
		}

		// Store provisioned user in context for downstream middleware and handlers
		ctx = platform.WithDomainUser(ctx, user)
		c.Request = c.Request.WithContext(ctx)

		// If an invitation activator is provided, try to activate pending invitations.
		// TenantID may not be in context yet (TenantFromHeader runs after), so this
		// hook is called post-tenant-resolution in invitation_usecase.go (T031).
		_ = activator // used in Phase 7 T031

		c.Next()
	}
}

// TenantFromHeader reads X-Tenant-ID header and validates the user's active membership.
// Skipped for paths that don't require a tenant (GET /api/v1/me, POST /api/v1/auth/change-password).
func TenantFromHeader(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Paths that do not require X-Tenant-ID
		if isExemptFromTenant(c.FullPath()) {
			c.Next()
			return
		}

		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Tenant-ID header"})
			return
		}

		// Validate user has an active role in this tenant
		user, ok := platform.DomainUser(c.Request.Context()).(*domain.User)
		if !ok || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		// Validate membership and load role in a single query
		var roleID string
		err := db.QueryRow(c.Request.Context(),
			`SELECT role_id FROM user_tenant_roles
			 WHERE user_id = $1 AND tenant_id = $2 AND status = 'active'
			 LIMIT 1`,
			user.ID, tenantID).Scan(&roleID)
		if err != nil {
			Log.Warn("tenant access denied",
				zap.String("user_id", user.ID),
				zap.String("tenant_id", tenantID),
				zap.String("endpoint", c.Request.URL.Path),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "tenant access denied"})
			return
		}

		ctx := platform.WithTenantID(c.Request.Context(), tenantID)
		ctx = security.WithRole(ctx, roleID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// PasswordChangeGuard blocks requests when user must change their password.
// Exempt paths: GET /api/v1/me and POST /api/v1/auth/change-password.
func PasswordChangeGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isExemptFromPasswordGuard(c.FullPath()) {
			c.Next()
			return
		}

		user, ok := platform.DomainUser(c.Request.Context()).(*domain.User)
		if ok && user != nil && user.PasswordChangeRequired {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "password_change_required"})
			return
		}
		c.Next()
	}
}

// RBACCheck returns a middleware that enforces the given permission.
func RBACCheck(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := security.Can(c.Request.Context(), perm); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// requestIDKey is the context key for the request ID.
type requestIDKey struct{}

// RequestID injects a unique request ID into the response header and request context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.New().String()
		c.Header("X-Request-ID", id)
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, requestIDKey{}, id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// Logger logs basic request info after the handler completes using Zap.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		Log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", c.Writer.Header().Get("X-Request-ID")),
		)
	}
}

// CORS sets permissive CORS headers for the API.
// TODO(deuda-tecnica): reemplazar "*" por una whitelist de orígenes leída desde config
// (CORS_ALLOWED_ORIGINS env var, separada por comas). El "*" es aceptable en desarrollo
// pero debe restringirse antes de ir a producción para prevenir CSRF.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Tenant-ID,X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}


func isExemptFromTenant(path string) bool {
	return path == "/api/v1/me" || path == "/api/v1/auth/change-password"
}

func isExemptFromPasswordGuard(path string) bool {
	return path == "/api/v1/me" || path == "/api/v1/auth/change-password"
}
