package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DBConfig struct {
	URL             string
	MaxConns        int
	MinConns        int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL string
}

type SupabaseConfig struct {
	JWKSUrl             string
	JWTIssuer           string
	JWTAudience         string
	URL                 string
	ServiceRoleKey      string
	AppBaseURL          string
	InviteRateLimitHour int
}

type ObservabilityConfig struct {
	LogLevel string
}

type Config struct {
	HTTP          HTTPConfig
	DB            DBConfig
	Redis         RedisConfig
	Supabase      SupabaseConfig
	Observability ObservabilityConfig
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second),
		},
		DB: DBConfig{
			URL:             requireEnv("DATABASE_URL"),
			MaxConns:        getIntEnv("DB_MAX_CONNS", 10),
			MinConns:        getIntEnv("DB_MIN_CONNS", 2),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", ""),
		},
		Supabase: SupabaseConfig{
			JWKSUrl:             requireEnv("SUPABASE_JWKS_URL"),
			JWTIssuer:           requireEnv("SUPABASE_JWT_ISSUER"),
			JWTAudience:         getEnv("SUPABASE_JWT_AUDIENCE", "authenticated"),
			URL:                 requireEnv("SUPABASE_URL"),
			ServiceRoleKey:      requireEnv("SUPABASE_SERVICE_ROLE_KEY"),
			AppBaseURL:          requireEnv("APP_BASE_URL"),
			InviteRateLimitHour: getIntEnv("INVITATION_RATE_LIMIT_PER_HOUR", 20),
		},
		Observability: ObservabilityConfig{
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "WARNING: required env var %s is not set\n", key)
	}
	return v
}

func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
