package telemetry

import "github.com/prometheus/client_golang/prometheus"

var (
	LogRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_requests_total",
			Help: "Total number of log API requests by operation and status.",
		},
		[]string{"operation", "status"},
	)

	LogListLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "log_list_duration_seconds",
			Help:    "Latency of log list operations in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	LogExportTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_export_total",
			Help: "Total number of log export requests.",
		},
		[]string{"truncated"},
	)
)

func init() {
	prometheus.MustRegister(
		LogRequestsTotal,
		LogListLatency,
		LogExportTotal,
	)
}
