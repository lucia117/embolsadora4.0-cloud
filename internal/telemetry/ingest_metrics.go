package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// IngestBatchesTotal cuenta batches procesados por desenlace (ok/error).
	IngestBatchesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_batches_total",
		Help: "Total de batches de eventos recibidos, por desenlace",
	}, []string{"status"})

	// IngestEventsAcceptedTotal cuenta eventos efectivamente persistidos.
	IngestEventsAcceptedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_events_accepted_total",
		Help: "Total de eventos aceptados y persistidos",
	})

	// IngestEventsRejectedTotal cuenta eventos rechazados por codigo.
	//
	// Es la metrica mas importante del endpoint: un salto de INVALID_SCHEMA o
	// VALIDATION_FAILED significa que el Edge esta mandando eventos a DEAD, y
	// eso es perdida de datos permanente. Merece alerta.
	IngestEventsRejectedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_events_rejected_total",
		Help: "Total de eventos rechazados, por codigo de error",
	}, []string{"code"})

	// IngestAuthTotal cuenta intentos de autenticacion por API key.
	IngestAuthTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_auth_total",
		Help: "Intentos de autenticacion por API key, por resultado",
	}, []string{"result"})

	// IngestRateLimitedTotal cuenta requests rechazados con 429.
	IngestRateLimitedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ingest_rate_limited_total",
		Help: "Total de requests rechazados por rate limit",
	})

	// IngestBatchDuration mide la latencia del insertMany.
	// Los buckets llegan hasta 2s porque SC-008 pide p95 < 500 ms con 1000
	// eventos: sin buckets por encima del objetivo, el p95 no se puede medir.
	IngestBatchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ingest_batch_duration_seconds",
		Help:    "Latencia de persistencia de un batch",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
	})

	// ingestMongoUp refleja el estado de la conexion a Mongo (1 arriba, 0 abajo).
	ingestMongoUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_mongo_up",
		Help: "1 si MongoDB responde al ping, 0 si no",
	})
)

// SetMongoUp registra el estado de Mongo. Lo llama el health check.
func SetMongoUp(up bool) {
	if up {
		ingestMongoUp.Set(1)
		return
	}
	ingestMongoUp.Set(0)
}
