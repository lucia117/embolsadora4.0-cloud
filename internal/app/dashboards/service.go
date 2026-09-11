// Package dashboards orquesta el motor de consultas de metricas: valida via
// domain/metrics, reparte las sub-consultas por MetricSpec (errgroup) y
// arma la respuesta segun el modo detectado.
package dashboards

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

type Service struct {
	repo   domain.Repository
	limits domain.Limits
	logger *zap.Logger
}

func NewService(repo domain.Repository, limits domain.Limits, logger *zap.Logger) *Service {
	return &Service{repo: repo, limits: limits, logger: logger}
}

// Query resuelve una MetricQuery completa: window -> validate -> fan-out por
// MetricSpec -> ensamblado por Mode. `now` se inyecta (no time.Now()
// interno) para que ResolveWindow sea testeable end-to-end.
func (s *Service) Query(ctx context.Context, tenantID string, q domain.MetricQuery, now time.Time) (domain.QueryResult, error) {
	from, to, err := domain.ResolveWindow(q, now)
	if err != nil {
		return domain.QueryResult{}, err
	}
	mode, err := domain.Validate(q, from, to, s.limits)
	if err != nil {
		return domain.QueryResult{}, err
	}

	result := domain.QueryResult{Mode: mode, MachineID: q.MachineID, From: from, To: to}

	switch mode {
	case domain.ModeRaw:
		return s.queryRaw(ctx, tenantID, q, from, to, result)
	case domain.ModeGrouped:
		return s.queryGrouped(ctx, tenantID, q, from, to, result)
	case domain.ModeSeries:
		return s.querySeries(ctx, tenantID, q, from, to, result)
	default:
		return s.queryScalar(ctx, tenantID, q, from, to, result)
	}
}

func (s *Service) queryScalar(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	results := make([]domain.MetricResult, len(q.Metrics))
	dataAsOfs := make([]*time.Time, len(q.Metrics))

	g, gctx := errgroup.WithContext(ctx)
	for i, spec := range q.Metrics {
		i, spec := i, spec
		g.Go(func() error {
			r, asOf, err := s.repo.Scalar(gctx, tenantID, q.MachineID, from, to, spec, q.Filter)
			if err != nil {
				return err
			}
			results[i] = r
			dataAsOfs[i] = asOf
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.QueryResult{}, err
	}

	result.Results = results
	result.DataAsOf = maxDataAsOf(dataAsOfs)
	return result, nil
}

func (s *Service) querySeries(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	perMetric := make([][]domain.BucketPoint, len(q.Metrics))
	dataAsOfs := make([]*time.Time, len(q.Metrics))

	g, gctx := errgroup.WithContext(ctx)
	for i, spec := range q.Metrics {
		i, spec := i, spec
		g.Go(func() error {
			points, asOf, err := s.repo.Series(gctx, tenantID, q.MachineID, from, to, q.Bucket, spec, q.Filter)
			if err != nil {
				return err
			}
			perMetric[i] = points
			dataAsOfs[i] = asOf
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.QueryResult{}, err
	}

	result.Bucket = q.Bucket
	result.Series = mergeSeries(q.Metrics, perMetric)
	result.DataAsOf = maxDataAsOf(dataAsOfs)
	return result, nil
}

// mergeSeries alinea por ts las series de cada metrica (una por MetricSpec,
// devueltas por Series/repo) en el shape combinado que pide la spec: un
// SeriesPoint por bucket, con un MetricResult por metrica presente en ese
// bucket. Un bucket sin dato para una metrica NO se rellena con 0 (spec).
func mergeSeries(specs []domain.MetricSpec, perMetric [][]domain.BucketPoint) []domain.SeriesPoint {
	byTs := make(map[time.Time][]domain.MetricResult)
	order := make([]time.Time, 0)
	seen := make(map[time.Time]bool)

	for i, points := range perMetric {
		spec := specs[i]
		for _, p := range points {
			byTs[p.Ts] = append(byTs[p.Ts], domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: p.Value, SampleCount: p.SampleCount})
			if !seen[p.Ts] {
				seen[p.Ts] = true
				order = append(order, p.Ts)
			}
		}
	}
	sortTimes(order)

	series := make([]domain.SeriesPoint, 0, len(order))
	for _, ts := range order {
		series = append(series, domain.SeriesPoint{Ts: ts, Results: byTs[ts]})
	}
	return series
}

func sortTimes(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

func (s *Service) queryRaw(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	spec := q.Metrics[0]
	limit := s.limits.MaxRawPoints + 1
	points, dataAsOf, err := s.repo.Raw(ctx, tenantID, q.MachineID, from, to, spec.AasPath, limit, q.MaxPoints)
	if err != nil {
		return domain.QueryResult{}, err
	}
	if q.MaxPoints <= 0 && len(points) >= limit {
		return domain.QueryResult{}, &domain.ValidationError{Code: domain.CodeRangeTooWide, Message: "el rango produce mas puntos crudos que el maximo permitido"}
	}

	result.AasPath = spec.AasPath
	result.Points = points
	result.DataAsOf = dataAsOf
	return result, nil
}

func (s *Service) queryGrouped(ctx context.Context, tenantID string, q domain.MetricQuery, from, to time.Time, result domain.QueryResult) (domain.QueryResult, error) {
	spec := q.Metrics[0]
	limit := s.limits.MaxGroups + 1
	groups, dataAsOf, err := s.repo.Grouped(ctx, tenantID, q.MachineID, from, to, q.GroupBy, spec, q.Filter, limit)
	if err != nil {
		return domain.QueryResult{}, err
	}
	if len(groups) >= limit {
		return domain.QueryResult{}, &domain.ValidationError{Code: domain.CodeTooManyGroups, Message: "groupBy produce mas grupos que el maximo permitido"}
	}

	result.AasPath = spec.AasPath
	result.Agg = spec.Agg
	result.GroupBy = q.GroupBy
	result.Groups = groups
	result.DataAsOf = dataAsOf
	return result, nil
}

func maxDataAsOf(ts []*time.Time) *time.Time {
	var max *time.Time
	for _, t := range ts {
		if t == nil {
			continue
		}
		if max == nil || t.After(*max) {
			max = t
		}
	}
	return max
}
