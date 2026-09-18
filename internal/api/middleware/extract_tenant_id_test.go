package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newExtractTenantIDRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/probe", ExtractTenantID(), func(c *gin.Context) {
		v, _ := c.Get(TenantID)
		c.JSON(http.StatusOK, gin.H{"tenant_id": v})
	})
	return r
}

func TestExtractTenantID_MissingHeader_Returns400(t *testing.T) {
	r := newExtractTenantIDRouter()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "MISSING_HEADER")
}

func TestExtractTenantID_InvalidUUID_Returns400(t *testing.T) {
	r := newExtractTenantIDRouter()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "INVALID_HEADER")
}

func TestExtractTenantID_ValidUUID_SetsContextAndCallsNext(t *testing.T) {
	r := newExtractTenantIDRouter()
	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("X-Tenant-ID", id)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), id)
}
