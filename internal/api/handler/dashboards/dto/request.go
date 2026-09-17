package dto

import (
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type MetricSpecDTO struct {
	AasPath string `json:"aasPath"`
	Agg     string `json:"agg"`
}

type ValueFilterDTO struct {
	ValueEquals any `json:"valueEquals,omitempty"`
}

type QueryRequest struct {
	MachineID string          `json:"machineId"`
	Range     string          `json:"range,omitempty"`
	From      *time.Time      `json:"from,omitempty"`
	To        *time.Time      `json:"to,omitempty"`
	Bucket    string          `json:"bucket,omitempty"`
	Metrics   []MetricSpecDTO `json:"metrics"`
	GroupBy   string          `json:"groupBy,omitempty"`
	Filter    *ValueFilterDTO `json:"filter,omitempty"`
	MaxPoints int             `json:"maxPoints,omitempty"`
}

// ToDomain traduce el DTO 1:1 -- no valida (eso lo hace domain/metrics
// dentro de app/dashboards.Service.Query, con los guardrails parametrizados
// por Limits).
func (r QueryRequest) ToDomain() domain.MetricQuery {
	specs := make([]domain.MetricSpec, len(r.Metrics))
	for i, m := range r.Metrics {
		specs[i] = domain.MetricSpec{AasPath: m.AasPath, Agg: domain.Agg(m.Agg)}
	}
	var filter *domain.ValueFilter
	if r.Filter != nil {
		filter = &domain.ValueFilter{ValueEquals: r.Filter.ValueEquals}
	}
	return domain.MetricQuery{
		MachineID: r.MachineID,
		Range:     domain.Range(r.Range),
		From:      r.From,
		To:        r.To,
		Bucket:    domain.Bucket(r.Bucket),
		Metrics:   specs,
		GroupBy:   r.GroupBy,
		Filter:    filter,
		MaxPoints: r.MaxPoints,
	}
}
