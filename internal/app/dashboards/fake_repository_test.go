package dashboards

import (
	"context"
	"time"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

// fakeRepository es un stub de domain.Repository a mano -- este repo no usa
// uber/mock en ningun lado (ver Global Constraints del plan), asi que esto
// sigue el mismo estilo que el resto del codigo.
type fakeRepository struct {
	scalarResult  domain.MetricResult
	scalarErr     error
	seriesResult  []domain.BucketPoint
	seriesErr     error
	rawResult     []domain.RawPoint
	rawErr        error
	groupedResult []domain.GroupResult
	groupedErr    error
	catalogPaths  []string
	catalogErr    error
}

func (f *fakeRepository) Scalar(context.Context, string, string, time.Time, time.Time, domain.MetricSpec, *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	if f.scalarErr != nil {
		return domain.MetricResult{}, nil, f.scalarErr
	}
	return f.scalarResult, nil, nil
}
func (f *fakeRepository) Series(context.Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	if f.seriesErr != nil {
		return nil, nil, f.seriesErr
	}
	return f.seriesResult, nil, nil
}
func (f *fakeRepository) Raw(context.Context, string, string, time.Time, time.Time, string, int, int) ([]domain.RawPoint, *time.Time, error) {
	if f.rawErr != nil {
		return nil, nil, f.rawErr
	}
	return f.rawResult, nil, nil
}
func (f *fakeRepository) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	if f.groupedErr != nil {
		return nil, nil, f.groupedErr
	}
	return f.groupedResult, nil, nil
}
func (f *fakeRepository) Catalog(context.Context, string, string) ([]string, error) {
	return f.catalogPaths, f.catalogErr
}
