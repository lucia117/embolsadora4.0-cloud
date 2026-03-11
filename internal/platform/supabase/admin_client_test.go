package supabase_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
)

func TestAdminClient_InviteUserByEmail_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/v1/admin/invite", r.URL.Path)
		assert.Equal(t, "Bearer test-service-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body["email"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "user-123"})
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.InviteUserByEmail(context.Background(), "user@example.com", "https://app.example.com/s/demo/auth/callback")
	require.NoError(t, err)
}

func TestAdminClient_InviteUserByEmail_4xxNoRetry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnprocessableEntity) // 422
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.InviteUserByEmail(context.Background(), "user@example.com", "https://app.example.com/callback")
	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "should NOT retry on 4xx")
}

func TestAdminClient_InviteUserByEmail_5xxRetry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError) // 500
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.InviteUserByEmail(context.Background(), "user@example.com", "https://app.example.com/callback")
	assert.Error(t, err)
	assert.Equal(t, 2, callCount, "should retry once on 5xx")
}

func TestAdminClient_SendPasswordResetEmail_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/v1/admin/generate-link", r.URL.Path)
		assert.Equal(t, "Bearer test-service-key", r.Header.Get("Authorization"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "recovery", body["type"])
		assert.Equal(t, "user@example.com", body["email"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"action_link": "https://supabase.co/reset?token=abc"})
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.SendPasswordResetEmail(context.Background(), "user@example.com")
	require.NoError(t, err)
}
