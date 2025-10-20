package telemetry

import (
    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetrics exposes Prometheus metrics at /metrics on the main router.
func RegisterMetrics(r *gin.Engine) {
    // TODO: customize registry and collectors if needed.
    r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
