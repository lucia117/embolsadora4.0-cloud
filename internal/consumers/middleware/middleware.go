package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/consumers"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// APIKeyAuth resuelve el header X-Api-Key a la identidad del device.
//
// Ante credencial invalida responde 403, no 401. El codigo esta fijado por el
// contrato: el Edge trata al 403 como "reintentar, probablemente haya una
// rotacion de key en curso". Un 401 no esta en su tabla y no aporta nada.
func APIKeyAuth(auth security.Authenticator, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Api-Key")
		if key == "" {
			telemetry.IngestAuthTotal.WithLabelValues("missing").Inc()
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "falta el header X-Api-Key"})
			return
		}

		identity, err := auth.Authenticate(c.Request.Context(), key)
		if err != nil {
			if errors.Is(err, security.ErrAPIKeyInvalid) {
				telemetry.IngestAuthTotal.WithLabelValues("invalid").Inc()
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "API key invalida"})
				return
			}
			// Postgres caido al resolver la key no es una credencial invalida.
			// Es un 500 para que el Edge reintente, no un 403 (I-1 aplicado a auth).
			log.Error("error resolviendo la API key", zap.Error(err))
			telemetry.IngestAuthTotal.WithLabelValues("error").Inc()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error de autenticacion"})
			return
		}

		telemetry.IngestAuthTotal.WithLabelValues("ok").Inc()
		c.Request = c.Request.WithContext(security.WithDeviceIdentity(c.Request.Context(), identity))
		c.Next()
	}
}

// RateLimit aplica el token bucket por key_id.
//
// Va DESPUES de APIKeyAuth para poder limitar por key y no por IP: todos los Pi
// de una misma planta pueden salir por la misma IP.
func RateLimit(limiter *consumers.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := security.DeviceIdentityFrom(c.Request.Context())
		if identity == nil {
			c.Next()
			return
		}

		allowed, retryAfter, _ := limiter.Allow(c.Request.Context(), identity.KeyID)
		if !allowed {
			telemetry.IngestRateLimitedTotal.Inc()
			// El Retry-After no es decorativo: el Edge lo lee y espera ese
			// tiempo exacto antes de reintentar.
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "rate limit excedido"})
			return
		}
		c.Next()
	}
}

// NoCORS marca la superficie de consumers como no navegable desde un browser:
// la consumen dispositivos, no paginas. No emitir headers CORS hace que
// cualquier preflight falle, que es exactamente lo que se quiere.
func NoCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		c.Next()
	}
}
