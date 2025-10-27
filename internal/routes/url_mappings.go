package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	api "github.com/tu-org/embolsadora-api/internal/api"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	consumers "github.com/tu-org/embolsadora-api/internal/consumers"
	consumermw "github.com/tu-org/embolsadora-api/internal/consumers/middleware"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// RegisterURLMappings configura todas las rutas de la API en un único lugar.
func RegisterURLMappings(r *gin.Engine) {
	// Health checks
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/readyz", func(c *gin.Context) { c.Status(http.StatusOK) })

	// Métricas Prometheus
	telemetry.RegisterMetrics(r)

	// Superficie administrativa / interna (v1)
	v1 := r.Group(
		"/api/v1",
		apimw.JWTAuth(),
		apimw.TenantFromJWT(),
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
	)
	api.RegisterAdminRoutes(v1, api.Deps{}, api.Config{})

	// Superficie para consumidores (IoT / dispositivos, etc.)
	c1 := r.Group(
		"/api/v1/consumers",
		consumermw.APIKeyAuth(),
		consumermw.RateLimit(),
		consumermw.Idempotency(),
		consumermw.NoCORS(),
		consumermw.Timeout(),
	)
	consumers.RegisterConsumerRoutes(c1, consumers.Deps{}, consumers.Config{})
}
