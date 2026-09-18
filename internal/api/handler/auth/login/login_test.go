package login

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandle_ValidCredentials_ProxiesSupabaseResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeSupabase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/auth/v1/token", r.URL.Path)
		require.Equal(t, "anon-key", r.Header.Get("apikey"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"tok123"}`))
	}))
	defer fakeSupabase.Close()

	h := NewHandler(fakeSupabase.URL, "anon-key")
	r := gin.New()
	r.POST("/auth/login", h.Handle)

	body := `{"email":"user@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "tok123")
}

func TestHandle_InvalidCredentials_ProxiesSupabase400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeSupabase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer fakeSupabase.Close()

	h := NewHandler(fakeSupabase.URL, "anon-key")
	r := gin.New()
	r.POST("/auth/login", h.Handle)

	body := `{"email":"user@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_grant")
}

func TestHandle_MalformedBody_Returns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler("http://unused.invalid", "anon-key")
	r := gin.New()
	r.POST("/auth/login", h.Handle)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"only-email"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandle_SupabaseUnreachable_Returns502 apunta a un httptest.Server ya
// cerrado: nadie escucha en esa URL, así que h.httpClient.Do falla igual que
// si Supabase estuviera caído, sin depender de una URL sintácticamente
// inválida que net/http podría rechazar en una etapa distinta.
func TestHandle_SupabaseUnreachable_Returns502(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeSupabase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := fakeSupabase.URL
	fakeSupabase.Close()

	h := NewHandler(unreachableURL, "anon-key")
	r := gin.New()
	r.POST("/auth/login", h.Handle)

	body := `{"email":"user@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code)
}
