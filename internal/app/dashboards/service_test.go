package dashboards

import (
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
