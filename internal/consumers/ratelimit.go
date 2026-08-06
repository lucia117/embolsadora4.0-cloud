package consumers

import (
	"context"
	"math"
	"time"

	"github.com/go-redis/redis/v8"
)

// tokenBucketScript implementa un token bucket en Redis.
//
// Va en Lua y no en Go porque leer-decidir-escribir desde la aplicacion tiene
// una condicion de carrera entre replicas: dos instancias podrian ver el mismo
// saldo de tokens y ambas dejar pasar. El script se ejecuta atomicamente.
//
// Devuelve {permitido, retry_after_segundos}.
const tokenBucketScript = `
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

// RateLimiter limita requests por API key con un token bucket distribuido.
type RateLimiter struct {
	rdb    *redis.Client
	rps    int
	burst  int
	script *redis.Script
}

// NewRateLimiter construye el limitador. `rdb` puede ser nil: sin Redis no se
// limita nada. Es la misma politica de fail-open que el resto del repo — un
// Redis caido no puede cortar la ingesta, porque el costo de rechazar datos
// reales es mayor que el de no limitar por un rato.
func NewRateLimiter(rdb *redis.Client, rps, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 200
	}
	if burst <= 0 {
		burst = 1000
	}
	return &RateLimiter{rdb: rdb, rps: rps, burst: burst, script: redis.NewScript(tokenBucketScript)}
}

// Allow consume un token para `key`. Devuelve si se permite el request y,
// si no, cuantos segundos esperar. Ante cualquier error de Redis permite pasar.
func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, int, error) {
	if l.rdb == nil {
		return true, 0, nil
	}
	now := timeNowMillis()
	res, err := l.script.Run(ctx, l.rdb, []string{"ratelimit:v1:" + key},
		l.rps, l.burst, now, 1).Result()
	if err != nil {
		return true, 0, err
	}
	vals, ok := res.([]any)
	if !ok || len(vals) != 2 {
		return true, 0, nil
	}
	allowed, _ := vals[0].(int64)
	retryAfter, _ := vals[1].(int64)
	return allowed == 1, int(math.Max(float64(retryAfter), 0)), nil
}

func timeNowMillis() int64 { return time.Now().UnixMilli() }
