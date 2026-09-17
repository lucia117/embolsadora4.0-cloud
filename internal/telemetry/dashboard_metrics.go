package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DashboardMetricsNonNumericDiscardedTotal cuenta valores de payload.value
// no numericos descartados por $isNumber en agregaciones avg/sum/min/max
// (fork 2 de la spec: el Edge no tiene garantia formal de que value sea
// siempre escalar, asi que esto es lo que hace observable un desajuste que
// de otro modo seria silencioso).
//
// Cardinalidad: tenant x aasPath. Aceptable con pocos tenants; si el numero
// de aasPath distintos crece mucho, revisar (ver spec, fork 2 "revisar
// cuando").
var DashboardMetricsNonNumericDiscardedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "dashboard_metrics_non_numeric_discarded_total",
	Help: "Valores de payload.value no numericos descartados en agregaciones avg/sum/min/max, por tenant/aasPath/agg",
}, []string{"tenant", "aas_path", "agg"})
