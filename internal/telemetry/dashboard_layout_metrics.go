package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DashboardLayoutRequestsTotal counts dashboard layout operations by operation and status.
	DashboardLayoutRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dashboard_layout_requests_total",
		Help: "Total number of dashboard layout requests by operation and status",
	}, []string{"operation", "status"})

	// DashboardLayoutRequestDuration measures latency of dashboard layout operations.
	DashboardLayoutRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dashboard_layout_request_duration_seconds",
		Help:    "Duration of dashboard layout requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})
)
