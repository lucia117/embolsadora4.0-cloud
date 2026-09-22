package edgeclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
)

func TestStatusCheck(t *testing.T) {
	t.Run("feliz: parsea el CheckResult del device", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/status", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"overallStatus": "OK",
				"checkedAt":     time.Now().Format(time.RFC3339),
			})
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.StatusCheck(t.Context(), srv.URL)
		require.NoError(t, err)
		assert.Equal(t, "OK", got.OverallStatus)
		assert.NotNil(t, got.Details, "Details nunca debe quedar nil aunque el device no lo mande")
	})

	t.Run("status no-2xx sintetiza un CheckResult ERROR en vez de fallar", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.StatusCheck(t.Context(), srv.URL)
		require.NoError(t, err, "un 503 del device no debe propagar error, se sintetiza un resultado")
		assert.Equal(t, "ERROR", got.OverallStatus)
		require.NotNil(t, got.Summary)
		assert.Contains(t, *got.Summary, "non-2xx")
	})

	t.Run("body invalido devuelve error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("no es json"))
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		_, err := c.StatusCheck(t.Context(), srv.URL)
		require.Error(t, err)
	})

	t.Run("timeout del cliente devuelve error de transporte", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(5 * time.Millisecond)
		_, err := c.StatusCheck(t.Context(), srv.URL)
		require.Error(t, err)
	})
}

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"overallStatus": "DEGRADED"})
	}))
	defer srv.Close()

	c := edgeclient.NewHTTPClient(2 * time.Second)
	got, err := c.HealthCheck(t.Context(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "DEGRADED", got.OverallStatus)
}

func TestGetTelemetry(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/telemetry", r.URL.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.GetTelemetry(t.Context(), srv.URL)
		require.NoError(t, err)
		assert.False(t, got.CapturedAt.IsZero(), "sin capturedAt en el JSON, debe rellenarse con now()")
	})

	t.Run("status no-2xx propaga error, no sintetiza (a diferencia de StatusCheck)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := edgeclient.NewHTTPClient(2 * time.Second)
		got, err := c.GetTelemetry(t.Context(), srv.URL)
		require.Error(t, err)
		assert.Nil(t, got)
	})
}

func TestNewHTTPClientTimeoutPorDefecto(t *testing.T) {
	// timeout=0 debe caer al default de 10s en vez de quedar sin límite;
	// lo verificamos indirectamente: un server que tarda 20ms debe responder bien.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"overallStatus": "OK"})
	}))
	defer srv.Close()

	c := edgeclient.NewHTTPClient(0)
	got, err := c.StatusCheck(t.Context(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "OK", got.OverallStatus)
}
