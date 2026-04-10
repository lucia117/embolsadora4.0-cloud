package telemetry

import "github.com/prometheus/client_golang/prometheus"

var (
	NotificationOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_operations_total",
			Help: "Total de operaciones en el servicio de notificaciones.",
		},
		[]string{"operation", "status"},
	)

	NotificationOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_operation_duration_seconds",
			Help:    "Latencia de operaciones de notificación en segundos.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(
		NotificationOperationsTotal,
		NotificationOperationDuration,
	)
}
