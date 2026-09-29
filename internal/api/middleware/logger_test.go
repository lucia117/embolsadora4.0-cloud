package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// withCapturedLog reemplaza el logger de paquete por uno que graba en memoria,
// corre fn, y lo restaura al final -- Log es una var de paquete compartida por
// todos los tests de este archivo, así que mutarla sin restaurar contaminaría
// corridas posteriores.
func withCapturedLog(t *testing.T, fn func()) *observer.ObservedLogs {
	t.Helper()
	original := Log
	core, logs := observer.New(zapcore.DebugLevel)
	SetLogger(zap.New(core))
	t.Cleanup(func() { Log = original })

	fn()
	return logs
}

func TestLogger_LogsRequestInfoAfterHandlerCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Logger())
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusTeapot) })

	logs := withCapturedLog(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusTeapot, w.Code)
	})

	entries := logs.FilterMessage("request").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "GET", fields["method"])
	require.Equal(t, "/probe", fields["path"])
	require.EqualValues(t, http.StatusTeapot, fields["status"])
	require.NotEmpty(t, fields["request_id"])
}

func TestSetLogger_NilArgument_IsIgnored(t *testing.T) {
	original := Log
	t.Cleanup(func() { Log = original })

	sentinel := zap.NewNop()
	Log = sentinel
	SetLogger(nil)

	require.Same(t, sentinel, Log, "SetLogger(nil) no debe reemplazar el logger actual")
}

func TestSetLogger_ValidLogger_Replaces(t *testing.T) {
	original := Log
	t.Cleanup(func() { Log = original })

	replacement := zap.NewNop()
	SetLogger(replacement)

	require.Same(t, replacement, Log)
}
