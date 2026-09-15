package dto

import (
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type MetricResultDTO struct {
	AasPath     string  `json:"aasPath"`
	Agg         string  `json:"agg"`
	Value       float64 `json:"value"`
	SampleCount int64   `json:"sampleCount"`
}

type SeriesPointDTO struct {
	Ts      time.Time         `json:"ts"`
	Results []MetricResultDTO `json:"results"`
}

type RawPointDTO struct {
	Ts    time.Time `json:"ts"`
	Value any       `json:"value"`
}

type GroupResultDTO struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// QueryResponse es la union de las 4 formas, discriminada por Mode (fork/C5)
// -- los campos que no aplican al modo detectado quedan omitidos via
// omitempty, nunca en su zero value visible.
type QueryResponse struct {
	Mode      string            `json:"mode"`
	MachineID string            `json:"machineId"`
	From      time.Time         `json:"from"`
	To        time.Time         `json:"to"`
	DataAsOf  *time.Time        `json:"dataAsOf"`
	Bucket    string            `json:"bucket,omitempty"`
	Results   []MetricResultDTO `json:"results,omitempty"`
	Series    []SeriesPointDTO  `json:"series,omitempty"`
	AasPath   string            `json:"aasPath,omitempty"`
	Points    []RawPointDTO     `json:"points,omitempty"`
	Agg       string            `json:"agg,omitempty"`
	GroupBy   string            `json:"groupBy,omitempty"`
	Groups    []GroupResultDTO  `json:"groups,omitempty"`
}

func QueryResultToResponse(r domain.QueryResult) QueryResponse {
	resp := QueryResponse{
		Mode:      string(r.Mode),
		MachineID: r.MachineID,
		From:      r.From,
		To:        r.To,
		DataAsOf:  r.DataAsOf,
		Bucket:    string(r.Bucket),
		AasPath:   r.AasPath,
		Agg:       string(r.Agg),
		GroupBy:   r.GroupBy,
	}
	for _, res := range r.Results {
		resp.Results = append(resp.Results, MetricResultDTO{AasPath: res.AasPath, Agg: string(res.Agg), Value: res.Value, SampleCount: res.SampleCount})
	}
	for _, sp := range r.Series {
		var results []MetricResultDTO
		for _, res := range sp.Results {
			results = append(results, MetricResultDTO{AasPath: res.AasPath, Agg: string(res.Agg), Value: res.Value, SampleCount: res.SampleCount})
		}
		resp.Series = append(resp.Series, SeriesPointDTO{Ts: sp.Ts, Results: results})
	}
	for _, p := range r.Points {
		resp.Points = append(resp.Points, RawPointDTO{Ts: p.Ts, Value: p.Value})
	}
	for _, g := range r.Groups {
		resp.Groups = append(resp.Groups, GroupResultDTO{Key: g.Key, Value: g.Value})
	}
	return resp
}
