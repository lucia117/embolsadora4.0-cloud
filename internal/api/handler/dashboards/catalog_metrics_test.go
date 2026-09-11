package dashboards

import (
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

func TestCatalogMetrics_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRepo{}
	repo.catalog = []string{"peso", "temperatura"}
	svc := app.NewService(repo, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.GET("/catalog", CatalogMetrics(svc))

	req := httptest.NewRequest(http.MethodGet, "/catalog?machineId=EMB-DEV-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	require.ElementsMatch(t, []any{"peso", "temperatura"}, data["aasPaths"])
}

func TestCatalogMetrics_MissingMachineID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := app.NewService(&fakeRepo{}, domain.DefaultLimits(), zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantID(c.Request.Context(), "tenant-1"))
		c.Next()
	})
	r.GET("/catalog", CatalogMetrics(svc))

	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
