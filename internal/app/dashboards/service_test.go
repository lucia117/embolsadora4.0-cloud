package dashboards

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

func TestService_Query_Scalar(t *testing.T) {
	repo := &fakeRepository{scalarResult: domain.MetricResult{AasPath: "peso", Agg: domain.AggAvg, Value: 1.5, SampleCount: 10}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}},
	}

	result, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.NoError(t, err)
	require.Equal(t, domain.ModeScalar, result.Mode)
	require.Len(t, result.Results, 1)
	require.Equal(t, 1.5, result.Results[0].Value)
}

func TestService_Query_PropagatesValidationError(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.DefaultLimits(), zap.NewNop())

	_, err := svc.Query(t.Context(), "tenant-1", domain.MetricQuery{}, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeInvalidParams, ve.Code)
}

func TestService_Query_FailsWholeQueryOnPartialRepoError(t *testing.T) {
	repo := &fakeRepository{scalarErr: assertAnError}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics: []domain.MetricSpec{
			{AasPath: "peso", Agg: domain.AggAvg},
			{AasPath: "temperatura", Agg: domain.AggAvg},
		},
	}
	_, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.Error(t, err) // fork/C7: first error wins, no partial scalar response
}

var assertAnError = errTest{}

type errTest struct{}

func (errTest) Error() string { return "mongo down" }

func TestService_Catalog(t *testing.T) {
	repo := &fakeRepository{catalogPaths: []string{"peso", "temperatura"}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	paths, err := svc.Catalog(t.Context(), "tenant-1", "EMB-DEV-001")
	require.NoError(t, err)
	require.Equal(t, []string{"peso", "temperatura"}, paths)
}

func TestService_Catalog_RequiresMachineID(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.DefaultLimits(), zap.NewNop())

	_, err := svc.Catalog(t.Context(), "tenant-1", "")
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeInvalidParams, ve.Code)
}

func TestService_Batch_PartialFailureDoesNotAbortOthers(t *testing.T) {
	repo := &fakeRepository{scalarResult: domain.MetricResult{Value: 42}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	okQuery := domain.MetricQuery{MachineID: "EMB-DEV-001", Range: domain.Range8h, Metrics: []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}}}
	badQuery := domain.MetricQuery{} // falla Validate: INVALID_PARAMS

	results, err := svc.Batch(t.Context(), "tenant-1", []BatchItem{
		{ID: "widget-a", Query: okQuery},
		{ID: "widget-b", Query: badQuery},
	}, time.Now())
	require.NoError(t, err) // el batch en si se procesa

	require.Len(t, results, 2)
	byID := map[string]BatchItemResult{}
	for _, r := range results {
		byID[r.ID] = r
	}
	require.NoError(t, byID["widget-a"].Err)
	require.Equal(t, 42.0, byID["widget-a"].Result.Results[0].Value)
	require.Error(t, byID["widget-b"].Err)
}

// TestService_Query_Grouped_TooManyGroups cubre I7.1: cuando el repo
// devuelve limits.MaxGroups+1 grupos (senal de "hay mas de los que se
// pidieron", misma convencion que Raw), Service.Query debe rechazar con
// TOO_MANY_GROUPS en vez de devolver una respuesta truncada sin avisar.
func TestService_Query_Grouped_TooManyGroups(t *testing.T) {
	limits := domain.DefaultLimits()
	groups := make([]domain.GroupResult, limits.MaxGroups+1)
	for i := range groups {
		groups[i] = domain.GroupResult{Key: "g", Value: float64(i)}
	}
	repo := &fakeRepository{groupedResult: groups}
	svc := NewService(repo, limits, zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggCount}},
		GroupBy:   "payload.tipo",
	}
	_, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeTooManyGroups, ve.Code)
}

// TestService_Query_Raw_RangeTooWide cubre I7.2: cuando el repo devuelve
// limits.MaxRawPoints+1 puntos (el fetch llego al cap) y MaxPoints no fue
// pedido, Service.Query debe rechazar con RANGE_TOO_WIDE.
func TestService_Query_Raw_RangeTooWide(t *testing.T) {
	limits := domain.DefaultLimits()
	points := make([]domain.RawPoint, limits.MaxRawPoints+1)
	for i := range points {
		points[i] = domain.RawPoint{Ts: time.Now(), Value: float64(i)}
	}
	repo := &fakeRepository{rawResult: points}
	svc := NewService(repo, limits, zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggRaw}},
	}
	_, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeRangeTooWide, ve.Code)
}

// TestService_Query_Series_HappyPath cubre I7.3 (serie): assembla
// QueryResult.Series a partir de lo que devuelve el fake Series.
func TestService_Query_Series_HappyPath(t *testing.T) {
	ts1 := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(time.Hour)
	repo := &fakeRepository{seriesResult: []domain.BucketPoint{
		{Ts: ts1, Value: 1.0, SampleCount: 2},
		{Ts: ts2, Value: 3.0, SampleCount: 1},
	}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}},
		Bucket:    domain.Bucket1h,
	}
	result, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.NoError(t, err)
	require.Equal(t, domain.ModeSeries, result.Mode)
	require.Len(t, result.Series, 2)
	require.Equal(t, ts1, result.Series[0].Ts)
	require.Equal(t, 1.0, result.Series[0].Results[0].Value)
	require.Equal(t, ts2, result.Series[1].Ts)
	require.Equal(t, 3.0, result.Series[1].Results[0].Value)
}

// TestService_Query_Grouped_HappyPath cubre I7.3 (grouped): assembla
// QueryResult.Groups a partir de lo que devuelve el fake Grouped.
func TestService_Query_Grouped_HappyPath(t *testing.T) {
	repo := &fakeRepository{groupedResult: []domain.GroupResult{
		{Key: "sellado_defectuoso", Value: 5},
		{Key: "temperatura_alta", Value: 2},
	}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "Alarmas/tipo", Agg: domain.AggCount}},
		GroupBy:   "payload.tipo",
	}
	result, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.NoError(t, err)
	require.Equal(t, domain.ModeGrouped, result.Mode)
	require.Equal(t, "payload.tipo", result.GroupBy)
	require.Len(t, result.Groups, 2)
	require.Equal(t, "sellado_defectuoso", result.Groups[0].Key)
}

// TestService_Query_Raw_HappyPath cubre I7.3 (raw), por simetria con
// series/grouped.
func TestService_Query_Raw_HappyPath(t *testing.T) {
	now := time.Now()
	repo := &fakeRepository{rawResult: []domain.RawPoint{
		{Ts: now, Value: 1.0},
		{Ts: now.Add(time.Second), Value: 2.0},
	}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	q := domain.MetricQuery{
		MachineID: "EMB-DEV-001",
		Range:     domain.Range8h,
		Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggRaw}},
	}
	result, err := svc.Query(t.Context(), "tenant-1", q, time.Now())
	require.NoError(t, err)
	require.Equal(t, domain.ModeRaw, result.Mode)
	require.Equal(t, "peso", result.AasPath)
	require.Len(t, result.Points, 2)
}

// TestService_Batch_ConcurrencyLimitPreservesAllResults cubre I6: con
// g.SetLimit(10) en Batch, un batch con mas items que el limite (15) debe
// seguir devolviendo los 15 resultados, correctamente correlacionados por
// ID -- esto guarda contra un error de wiring en SetLimit que descarte
// items silenciosamente.
func TestService_Batch_ConcurrencyLimitPreservesAllResults(t *testing.T) {
	repo := &fakeRepository{scalarResult: domain.MetricResult{Value: 42}}
	svc := NewService(repo, domain.DefaultLimits(), zap.NewNop())

	const n = 15
	items := make([]BatchItem, n)
	for i := 0; i < n; i++ {
		items[i] = BatchItem{
			ID: fmt.Sprintf("widget-%d", i),
			Query: domain.MetricQuery{
				MachineID: "EMB-DEV-001",
				Range:     domain.Range8h,
				Metrics:   []domain.MetricSpec{{AasPath: "peso", Agg: domain.AggAvg}},
			},
		}
	}

	results, err := svc.Batch(t.Context(), "tenant-1", items, time.Now())
	require.NoError(t, err)
	require.Len(t, results, n)

	byID := map[string]BatchItemResult{}
	for _, r := range results {
		byID[r.ID] = r
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("widget-%d", i)
		r, ok := byID[id]
		require.True(t, ok, "falta resultado para %s", id)
		require.NoError(t, r.Err)
		require.Equal(t, 42.0, r.Result.Results[0].Value)
	}
}

func TestService_Batch_RejectsTooManyQueries(t *testing.T) {
	svc := NewService(&fakeRepository{}, domain.Limits{MaxBatchQueries: 1, MaxSpecs: 10}, zap.NewNop())

	_, err := svc.Batch(t.Context(), "tenant-1", []BatchItem{
		{ID: "a", Query: domain.MetricQuery{}},
		{ID: "b", Query: domain.MetricQuery{}},
	}, time.Now())
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.CodeTooManyQueries, ve.Code)
}
