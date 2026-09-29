package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORS_SetsExpectedHeadersOnGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET,POST,PUT,PATCH,DELETE,OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	require.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-Tenant-ID")
}

func TestCORS_OptionsRequest_ShortCircuitsWith204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	nextCalled := false
	r.Use(CORS())
	r.OPTIONS("/probe", func(c *gin.Context) { nextCalled = true })

	req := httptest.NewRequest(http.MethodOptions, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	require.False(t, nextCalled, "un preflight OPTIONS no debe llegar al handler de la ruta")
}
