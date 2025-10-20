package main

import (
    "net/http"

    "github.com/gin-gonic/gin"

    api "github.com/tu-org/embolsadora-api/internal/api"
    apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
    consumers "github.com/tu-org/embolsadora-api/internal/consumers"
    consumermw "github.com/tu-org/embolsadora-api/internal/consumers/middleware"
    "github.com/tu-org/embolsadora-api/internal/telemetry"
)

// TODO: Bootstrap config, telemetry, repositories, services, and dependency wiring.
func main() {
    r := gin.New()

    // Global middlewares: RequestID and Logger (stubs for now)
    r.Use(apimw.RequestID())
    r.Use(apimw.Logger())

    r.GET("/healthz", func(c *gin.Context) {
        c.Status(http.StatusOK)
    })
    r.GET("/readyz", func(c *gin.Context) {
        c.Status(http.StatusOK)
    })

    // Expose Prometheus metrics at /metrics
    telemetry.RegisterMetrics(r)

    // Register API surfaces (stub routes return 501)
    v1 := r.Group(
        "/api/v1",
        apimw.JWTAuth(),
        apimw.TenantFromJWT(),
        apimw.RequestID(),
        apimw.Logger(),
        apimw.CORS(),
    )
    api.RegisterAdminRoutes(v1, api.Deps{}, api.Config{})

    c1 := r.Group(
        "/api/v1/consumers",
        consumermw.APIKeyAuth(),
        consumermw.RateLimit(),
        consumermw.Idempotency(),
        consumermw.NoCORS(),
        consumermw.Timeout(),
    )
    consumers.RegisterConsumerRoutes(c1, consumers.Deps{}, consumers.Config{})

    // TODO: get address from config. Hardcoded for dev.
    _ = r.Run(":8080")
}
