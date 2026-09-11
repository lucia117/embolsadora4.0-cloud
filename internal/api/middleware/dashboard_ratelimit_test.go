package middleware

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
