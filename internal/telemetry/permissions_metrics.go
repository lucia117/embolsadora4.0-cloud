package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PermissionsRequestsTotal counts permissions operations by operation and status.
	PermissionsRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "permissions_requests_total",
		Help: "Total number of permissions requests by operation and status",
	}, []string{"operation", "status"})

	// PermissionsListDuration measures latency of GET /permissions list operation.
	PermissionsListDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "permissions_list_duration_seconds",
		Help:    "Duration of permissions list requests in seconds",
		Buckets: prometheus.DefBuckets,
	})
)
