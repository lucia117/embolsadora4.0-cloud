package dashboards

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestBatchQueryMetrics_PartialSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{result: domain.MetricResult{Value: 42}}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.POST("/query/batch", BatchQueryMetrics(svc))

	body, _ := json.Marshal(map[string]any{
		"queries": []map[string]any{
			{"id": "widget-a", "machineId": "EMB-DEV-001", "range": "8h", "metrics": []map[string]any{{"aasPath": "peso", "agg": "avg"}}},
			{"id": "widget-b", "machineId": "EMB-DEV-001", "metrics": []map[string]any{{"aasPath": "x", "agg": "avg"}}}, // sin range ni from/to -> falla
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/query/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"])

	results := resp["data"].(map[string]any)["results"].([]any)
	require.Len(t, results, 2)

	byID := map[string]map[string]any{}
	for _, r := range results {
		item := r.(map[string]any)
		byID[item["id"].(string)] = item
	}
	require.Equal(t, true, byID["widget-a"]["success"])
	require.Equal(t, false, byID["widget-b"]["success"])
	require.Equal(t, "INVALID_PARAMS", byID["widget-b"]["code"])
}
