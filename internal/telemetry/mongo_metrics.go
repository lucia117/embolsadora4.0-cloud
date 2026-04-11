package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// MongoOperationDuration tracks latency of MongoDB repository operations by collection and operation name.
	MongoOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mongo_operation_duration_seconds",
		Help:    "Latency of MongoDB repository operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"collection", "operation"})

	// MongoOperationErrors counts MongoDB repository errors by collection and operation name.
	MongoOperationErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mongo_operation_errors_total",
		Help: "Total number of MongoDB operation errors",
	}, []string{"collection", "operation"})
)
