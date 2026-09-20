package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestParseDashboardBucketReply(t *testing.T) {
	tests := []struct {
		name           string
		res            any
		wantAllowed    bool
		wantRetryAfter int
		wantOK         bool
	}{
		{name: "permitido", res: []any{int64(1), int64(0)}, wantAllowed: true, wantOK: true},
		{name: "denegado con retry_after", res: []any{int64(0), int64(3)}, wantAllowed: false, wantRetryAfter: 3, wantOK: true},
		{name: "malformado -- debe abrir, no denegar", res: "no-es-slice", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, retryAfter, ok := parseDashboardBucketReply(tt.res)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAllowed, allowed)
				assert.Equal(t, tt.wantRetryAfter, retryAfter)
			}
		})
	}
}

func TestDashboardRateLimiter_AllowsWhenRedisNil(t *testing.T) {
	limiter := NewDashboardRateLimiter(nil, 15.0/60.0, 5)
	allowed, retryAfter, err := limiter.Allow(t.Context(), "user-1")
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, retryAfter)
}

// TestDashboardRateLimiter_DeniesSecondCallOverBurst cubre I7.4: el camino
// 429 real contra Redis, nunca antes ejercitado (los tests anteriores solo
// cubren parseDashboardBucketReply y el fail-open con rdb nil). burst=1
// significa que el segundo Allow con la misma key dentro de la ventana debe
// denegar con un retryAfter positivo. La key incorpora t.Name()+tiempo para
// no colisionar con corridas anteriores contra el mismo Redis de larga vida
// (mismo patron que internal/consumers/middleware/middleware_test.go).
func TestDashboardRateLimiter_DeniesSecondCallOverBurst(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit contra Redis real")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	key := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	limiter := NewDashboardRateLimiter(rdb, 0.01, 1) // rps bajo: el token no se recarga durante el test

	ctx := context.Background()
	allowed1, _, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed1, "el primer consumo debe pasar (burst=1)")

	allowed2, retryAfter2, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, allowed2, "el segundo consumo inmediato debe ser denegado (burst agotado)")
	assert.Greater(t, retryAfter2, 0)
}

func TestDashboardRateLimit_NoSupabaseSubInContext_CallsNextWithoutLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// rdb nil ya abriría (fail-open) igual, pero esto prueba la rama explícita
	// de "sin sub no hay a quién limitar" antes de siquiera llamar a Allow.
	r.Use(DashboardRateLimit(NewDashboardRateLimiter(nil, 15.0/60.0, 5)))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDashboardRateLimit_RedisNil_FailsOpenAndCallsNext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithSupabaseSub(c.Request.Context(), "user-1"))
		c.Next()
	})
	r.Use(DashboardRateLimit(NewDashboardRateLimiter(nil, 15.0/60.0, 5)))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Retry-After"))
}

// TestDashboardRateLimit_Denied_Returns429WithRetryAfter cubre la rama 429 del
// handler (no solo de Allow, ya cubierto arriba) contra Redis real -- mismo
// gate por REDIS_URL que TestDashboardRateLimiter_DeniesSecondCallOverBurst.
func TestDashboardRateLimit_Denied_Returns429WithRetryAfter(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit contra Redis real")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	key := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	limiter := NewDashboardRateLimiter(rdb, 0.01, 1) // rps bajo: el token no se recarga durante el test

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithSupabaseSub(c.Request.Context(), key))
		c.Next()
	})
	r.Use(DashboardRateLimit(limiter))
	r.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.Equal(t, http.StatusOK, w1.Code, "el primer request debe pasar (burst=1)")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/probe", nil))
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))
}
