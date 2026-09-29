package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesIDAndSetsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var idInContext string
	r.Use(RequestID())
	r.GET("/probe", func(c *gin.Context) {
		idInContext = RequestIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	header := w.Header().Get("X-Request-ID")
	require.NotEmpty(t, header)
	require.Equal(t, header, idInContext, "el ID en el header y el que ve el handler downstream deben coincidir")
}

func TestRequestID_TwoRequests_GetDifferentIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/probe", nil))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/probe", nil))

	require.NotEqual(t, w1.Header().Get("X-Request-ID"), w2.Header().Get("X-Request-ID"))
}

func TestRequestIDFromContext_NoIDSet_ReturnsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var idInContext string
	found := true
	r.GET("/probe", func(c *gin.Context) {
		idInContext = RequestIDFromContext(c.Request.Context())
		found = idInContext != ""
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.False(t, found, "sin pasar por RequestID(), el contexto no debe tener un request ID")
	require.Empty(t, idInContext)
}
