package dashboards

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type fakeRepo struct {
	result  domain.MetricResult
	catalog []string
}

func (f *fakeRepo) Scalar(_ context.Context, _, _ string, _, _ time.Time, spec domain.MetricSpec, _ *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	f.result.AasPath, f.result.Agg = spec.AasPath, spec.Agg
	return f.result, nil, nil
}
func (f *fakeRepo) Series(context.Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Raw(context.Context, string, string, time.Time, time.Time, string, *domain.ValueFilter, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeRepo) Catalog(context.Context, string, string) ([]string, error) { return f.catalog, nil }

func TestQueryMetrics_ScalarHappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{result: domain.MetricResult{Value: 1.5, SampleCount: 10}}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "11111111-1111-1111-1111-111111111111"))
		c.Next()
	})
	r.POST("/query", QueryMetrics(svc))

	body, _ := json.Marshal(map[string]any{
		"machineId": "EMB-DEV-001",
		"range":     "8h",
		"metrics":   []map[string]any{{"aasPath": "peso", "agg": "avg"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"])
	data := resp["data"].(map[string]any)
	require.Equal(t, "scalar", data["mode"])
}

func TestNormalizedTenantID(t *testing.T) {
	ctx := platform.WithTenantID(context.Background(), "11B36B85-033D-4BB3-9E31-4C92161887C0")
	id, ok := normalizedTenantID(ctx)
	require.True(t, ok)
	require.Equal(t, "11b36b85-033d-4bb3-9e31-4c92161887c0", id)

	_, ok = normalizedTenantID(platform.WithTenantID(context.Background(), "not-a-uuid"))
	require.False(t, ok)

	_, ok = normalizedTenantID(context.Background())
	require.False(t, ok)
}

// TestQueryMetrics_UppercaseTenantIDNormalized cubre I1: un X-Tenant-ID en
// mayusculas (pasa JWTAuth/membership/RBAC porque esos comparan contra una
// columna uuid de Postgres, case-insensitive) debe llegar al repo ya
// normalizado a minusculas -- sin esto, el $match byte-exacto de Mongo
// devuelve 200 con resultados vacios en vez de encontrar los datos guardados
// en minuscula.
func TestQueryMetrics_UppercaseTenantIDNormalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{result: domain.MetricResult{Value: 1.5, SampleCount: 10}}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "11B36B85-033D-4BB3-9E31-4C92161887C0"))
		c.Next()
	})
	r.POST("/query", QueryMetrics(svc))

	body, _ := json.Marshal(map[string]any{
		"machineId": "EMB-DEV-001",
		"range":     "8h",
		"metrics":   []map[string]any{{"aasPath": "peso", "agg": "avg"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestQueryMetrics_InvalidParamsReturns400WithCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := app.NewService(&fakeRepo{}, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "11111111-1111-1111-1111-111111111111"))
		c.Next()
	})
	r.POST("/query", QueryMetrics(svc))

	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, false, resp["success"])
	require.Equal(t, "INVALID_PARAMS", resp["code"])
}
