package middleware

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/tu-org/embolsadora-api/internal/platform"
)

// dashboardTokenBucketScript es el mismo patron que
// internal/consumers/ratelimit.go (token bucket atomico en Lua), copiado en
// vez de reusado: ese tipo esta scopeado al dominio de ingesta de Edge
// (rps entero, key = API key) y este necesita rps fraccionario (15/min =
// 0.25/seg) con key = supabase_user_id -- generalizar el tipo existente
// tocaria un camino caliente en produccion (ingesta) por un beneficio de
// DRY chico.
const dashboardTokenBucketScript = `
local rate      = tonumber(ARGV[1])
local burst     = tonumber(ARGV[2])
local now       = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data   = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])
if tokens == nil then
  tokens = burst
  ts = now
end

local delta = math.max(0, now - ts) / 1000.0
tokens = math.min(burst, tokens + delta * rate)

local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', KEYS[1], math.ceil((burst / rate) * 1000) + 1000)

local retry_after = 0
if allowed == 0 then
  retry_after = math.ceil((requested - tokens) / rate)
  if retry_after < 1 then retry_after = 1 end
end

return {allowed, retry_after}
`

// DashboardRateLimiter es un token bucket atomico en Redis, keyed por
// supabase_user_id, para el surface de dashboard metrics (JWT-autenticado).
// Deliberadamente separado de internal/consumers/ratelimit.go -- ver
// comentario de dashboardTokenBucketScript.
type DashboardRateLimiter struct {
	rdb    *redis.Client
	rps    float64
	burst  int
	script *redis.Script
}

// NewDashboardRateLimiter construye el limitador. rdb nil = sin limite
// (fail-open, misma politica que el resto del repo -- ver
// internal/consumers/ratelimit.go).
func NewDashboardRateLimiter(rdb *redis.Client, rps float64, burst int) *DashboardRateLimiter {
	if rps <= 0 {
		rps = 15.0 / 60.0
	}
	if burst <= 0 {
		burst = 5
	}
	return &DashboardRateLimiter{rdb: rdb, rps: rps, burst: burst, script: redis.NewScript(dashboardTokenBucketScript)}
}

// Allow consulta el token bucket para key. Si rdb es nil, o si Redis
// responde algo inesperado, abre (permite) en vez de denegar -- un rate
// limiter caido nunca debe tumbar el dashboard.
func (l *DashboardRateLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	if l.rdb == nil {
		return true, 0, nil
	}
	now := time.Now().UnixMilli()
	res, err := l.script.Run(ctx, l.rdb, []string{"ratelimit:dashboards:v1:" + key}, l.rps, l.burst, now, 1).Result()
	if err != nil {
		return true, 0, err
	}
	allowed, retryAfter, ok := parseDashboardBucketReply(res)
	if !ok {
		return true, 0, nil
	}
	return allowed, retryAfter, nil
}

func parseDashboardBucketReply(res any) (allowed bool, retryAfter int, ok bool) {
	vals, isSlice := res.([]any)
	if !isSlice || len(vals) != 2 {
		return false, 0, false
	}
	a, okAllowed := vals[0].(int64)
	r, okRetry := vals[1].(int64)
	if !okAllowed || !okRetry {
		return false, 0, false
	}
	return a == 1, int(math.Max(float64(r), 0)), true
}

// DashboardRateLimit limita por supabase_user_id (JWTAuth ya lo dejo en
// contexto). Va despues de JWTAuth y RBACCheck en la cadena.
func DashboardRateLimit(limiter *DashboardRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := platform.SupabaseSub(c.Request.Context())
		if key == "" {
			c.Next()
			return
		}
		allowed, retryAfter, _ := limiter.Allow(c.Request.Context(), key)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "rate limit excedido", "code": "RATE_LIMITED"})
			return
		}
		c.Next()
	}
}
